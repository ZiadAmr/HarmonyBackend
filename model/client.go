/**
 * Structure to keep track of client details.
 */

package model

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// length of key in bytes
const KEYLEN = 64

type PublicKey [KEYLEN]byte

// *websocket.Conn, but only the methods that are being used here
// So that *websocket.Conn can be mocked.
type Conn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
}

type Client struct {
	// PRIVATE METHODS: not accessible outside current package
	publicKey *PublicKey
	// lock to prevent simultaneous writes to the websocket conn
	conn          Conn
	connWriteLock sync.Mutex
	// map of active transactions for this client; id -> transaction
	// should not access directly outside client.go
	transactions           map[[IDLEN]byte]*clientTransactionWrapper
	modifyTransactionsLock sync.Mutex
	// used to prevent new transactions being added after broken out of the main loop
	// must use modifyTransactionsLock when reading or editing
	disconnected bool
	// channels that should be closed byt the main loop
	// these channels cause transaction goroutines to return when closed.
	danglingChannels           []chan string
	modifyDanglingChannelsLock sync.Mutex
}

func MakeClient(conn Conn) Client {
	return Client{
		publicKey: nil, // initially unset. When set, it implies the client has been added to the hub.

		conn:         conn,
		transactions: make(map[[IDLEN]byte]*clientTransactionWrapper),
	}
}

func (c *Client) GetPublicKey() *PublicKey {
	return c.publicKey
}

// not allowed if already set
func (c *Client) SetPublicKey(pk *PublicKey) error {
	if c.publicKey != nil {
		return errors.New("public key already set")
	}
	c.publicKey = pk
	return nil
}

// a loop that demultiplexes messages and forwards them to correct handlers
func (c *Client) Route(hub *Hub, makeRoutine func() Routine) {

	for {

		// check to see if there are any dangling channels that were created in this goroutine which need to be closed
		c.closeDanglingChannels()

		// read from websocket (blocking)
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// the first IDLEN bytes represent the id of the transaction
		// which uniquely identifies the instance of the active routine that the message needs to be forwarded to.
		// if the routine instance number is unrecognized, create a new routine.
		if len(msgBytes) < IDLEN {
			fmt.Println("User sent a malformed message: " + string(msgBytes))
			continue
		}
		id := ([IDLEN]byte)(msgBytes[:IDLEN])

		// check if a transaction with this id exists already
		tWrapper, exists := c.transactions[id]
		// if so, pass the message to that transaction
		if exists {
			select {
			case tWrapper.fromCl <- string(msgBytes[IDLEN:]):
			default:
				c.writeTransactionMessage(tWrapper.id, `{"error":"Buffer is occupied, message ignored"}`)
			}
			continue
		}

		// otherwise create a new transaction
		tWrapperNew := c.wrapTransaction(c.newTransaction(makeRoutine()), id)

		// add to transaction list
		c.addTransaction(tWrapperNew)

		// route
		go c.routeTransaction(hub, tWrapperNew)

		// queue first message
		tWrapperNew.fromCl <- string(msgBytes[IDLEN:])
	}

	// breaks out here when the websocket is closed.
	c.close(hub)

}

func (c *Client) wrapTransaction(transaction *transaction, id [IDLEN]byte) *clientTransactionWrapper {
	roChan := make(chan RoutineOutput)
	pk := c.GetPublicKey()
	if pk != nil {
		transaction.pkToROChan[*pk] = roChan
	}
	return &clientTransactionWrapper{
		fromCl:      make(chan string, 1),
		roChan:      roChan,
		transaction: transaction,
		id:          id,
	}
}

func (c *Client) newTransaction(routine Routine) *transaction {
	return &transaction{
		pkToROChan: make(map[PublicKey](chan RoutineOutput)),
		routine:    routine,
	}
}

// Add a new transaction to the client.
// The channels in the transaction will be assosiated with the provided id in the client.
// Threadsafe.
func (c *Client) addTransaction(t *clientTransactionWrapper) error {

	_, idExists := c.transactions[t.id]
	if idExists {
		panic("Attempted to registed a transaction id that already exists!")
	}

	err := func() error {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()
		if c.disconnected {
			return errors.New("client has disconnected")
		} else {
			c.transactions[t.id] = t
			return nil
		}
	}()
	return err
}

// threadsafe
func (c *Client) deleteTransaction(id [IDLEN]byte) error {

	var tw *clientTransactionWrapper

	// delete the transaction, if it exists
	err := func() error {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()

		t0, exists := c.transactions[id]
		if !exists {
			return errors.New("transaction does not exist")
		}
		// save the wrapper so we can deal with the dangling channels
		tw = t0
		delete(c.transactions, id)
		return nil
	}()
	if err != nil {
		return err
	}

	// remove roChan from the inner transaction
	pk := c.publicKey
	if pk != nil {
		func() {
			defer tw.transaction.pkToROChanLock.Unlock()
			tw.transaction.pkToROChanLock.Lock()
			delete(tw.transaction.pkToROChan, *pk)
		}()
	}

	// If the roChan channel is closing, then it is because the routine explicity told this transaction to end.
	// Therefore the routine should know not to send further messages to this client
	// We don't need to worry as much here as with the fromCl, to which the user could potentially keep sending messages
	close(tw.roChan)

	// moves the channel to the dangling channels list.
	// mutex section above ensures that sinch we reach here, this is the only time that the specified transaction is being deleted
	// therefore we don't need to worry about duplicate channels ending up in the dangling channels list.
	func() {
		defer c.modifyDanglingChannelsLock.Unlock()
		c.modifyDanglingChannelsLock.Lock()
		c.danglingChannels = append(c.danglingChannels, tw.fromCl)

	}()

	return nil
}

func (c *Client) close(hub *Hub) {
	// set disconnected - prevent more transactions being added.
	func() {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()
		c.disconnected = true
	}()

	// delete all remaining transactions.
	// they might also try to delete themselves in their own goroutines,
	// but DeleteTransaction has synchronization to ensure that the transactions get deleted at most once.
	// also send a ClientClose message to the routines
	for tid, t := range c.transactions {
		// send close message to all routines still open
		if !t.status.done {
			func() {
				defer t.transaction.routineLock.Unlock()
				t.transaction.routineLock.Lock()
				if t.status.done /*check it hasn't changed*/ {
					return
				}
				ros := t.transaction.routine.Next(RoutineMsgType_ClientClose, c.GetPublicKey(), "")
				c.distributeRoutineOutputs(hub, t, ros)
			}()
		}
		c.deleteTransaction(tid)
	}
	c.closeDanglingChannels()
}

// should run in a separate goroutine to the main Route() loop.
// this can ONLY be terminated by closing fromClMsg.
func (c *Client) routeTransaction(hub *Hub, t *clientTransactionWrapper) {

	t.status = transactionStatus{
		done:         false,
		timeoutTimer: nil,
	}

	for {
		select {

		// output from routine
		case ro := <-t.roChan:
			if t.status.done {
				continue
			}
			// routine output received from another goroutine
			t.status = c.processRoutineOutput(t, ro)
			if ro.Done {
				c.deleteTransaction(t.id) // this also adds t.fromCl to the dangling channels list
			}

		// timeout
		case <-t.status.timeoutTimer:

			if t.status.done {
				continue
			}

			go func() {
				defer t.transaction.routineLock.Unlock()
				t.transaction.routineLock.Lock()

				// status might have changed while waiting for the lock
				// check again
				if t.status.done {
					return
				}
				ros := t.transaction.routine.Next(RoutineMsgType_Timeout, c.GetPublicKey(), "")

				c.distributeRoutineOutputs(hub, t, ros)
			}()

		// message from client
		case fromClMsg, ok := <-t.fromCl:
			if !ok {
				// terminate this fn only when fromCl closes.
				// which is triggered by .deleteTransaction()
				return
			}
			if t.status.done {
				// the routine has ended and the Transaction struct has been deleted,
				// but the main Route loop hasn't figured that out yet and is continuing to send us messages.
				// the next time Route gets to the top of its loop it should close fromClMsg.
				// ignore message, and keep waiting for fromClMsg to be closed.
				c.writeTransactionMessage(t.id, `{"error":"transaction has terminated"}`)
				continue
			}

			// do this bit in a new goroutine
			// because of 1) the mutex, just to speed things up so there's no waiting, but also
			// 2) the last bit could lead to a deadlock without it, because two routines could be trying to send an RO to the other at the same time
			// note, the
			go func() {
				// call .Next()
				defer t.transaction.routineLock.Unlock()
				t.transaction.routineLock.Lock()
				// status might have changed while waiting for the lock
				if t.status.done {
					c.writeTransactionMessage(t.id, `{"error":"transaction has terminated"}`)
					return
				}
				ros := t.transaction.routine.Next(RoutineMsgType_UsrMsg, c.GetPublicKey(), fromClMsg)

				c.distributeRoutineOutputs(hub, t, ros)
			}()

		}

	}

}

// send routine outputs to correct clients.
// should be run in an independent goroutine of the routeTransaction loop.
func (c *Client) distributeRoutineOutputs(hub *Hub, t *clientTransactionWrapper, ros []RoutineOutput) {
	for _, routineOutput := range ros {
		if routineOutput.Pk == nil {
			// hope that roChan is not closed!
			// this would only occur if there's an error in the Routine,
			// - where it tries to send another RO after a previous one had done=true
			t.roChan <- routineOutput
		} else {
			roChan, exists := t.transaction.pkToROChan[*routineOutput.Pk]
			if exists {
				roChan <- routineOutput
			} else {
				// todo need to create a new transaction if it does not exist
				peerClient, exists := hub.GetClient(*routineOutput.Pk)
				if !exists {
					fmt.Printf("client does not exist")
					continue
				}
				tWrapper := peerClient.wrapTransaction(t.transaction, newId())
				err := peerClient.addTransaction(tWrapper)
				if err == nil {
					go peerClient.routeTransaction(hub, tWrapper)
					tWrapper.roChan <- routineOutput
				}
			}

		}
	}
}

// process RO for THIS client.
func (c *Client) processRoutineOutput(t *clientTransactionWrapper, ro RoutineOutput) transactionStatus {
	status := transactionStatus{}

	for _, toClMsg := range ro.Msgs {
		// write message
		err := c.writeTransactionMessage(t.id, toClMsg)
		if err != nil {
			fmt.Printf("Error writing message: " + err.Error())
		}
	}
	// set the timeout
	if ro.TimeoutEnabled {
		status.timeoutTimer = time.After(ro.TimeoutDuration)
	}

	status.done = ro.Done

	return status
}

// should only be called from the goroutine containing the main loop, as it closes channels created in the main loop
// this causes the transaction goroutines to terminate.
func (c *Client) closeDanglingChannels() {
	defer c.modifyDanglingChannelsLock.Unlock()
	c.modifyDanglingChannelsLock.Lock()

	if len(c.danglingChannels) > 0 {

		for _, ch := range c.danglingChannels {
			close(ch)
		}
		c.danglingChannels = make([]chan string, 0)
	}
}

// thread safe
func (c *Client) writeTransactionMessage(transactionID [IDLEN]byte, msg string) error {
	// concatenate transactionID and msg
	msgWithId := append(transactionID[:], []byte(msg)...)
	defer c.connWriteLock.Unlock()
	c.connWriteLock.Lock()
	return c.conn.WriteMessage(websocket.TextMessage, msgWithId)
}
