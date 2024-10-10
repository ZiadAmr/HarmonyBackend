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

// routine input buffer size
const RI_BUFFER_SIZE = 10

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
	transactions           map[[IDLEN]byte]*transactionSocket
	modifyTransactionsLock sync.Mutex
	// used to prevent new transactions being added after broken out of the main loop
	// must use modifyTransactionsLock when reading or editing
	disconnected bool
	// channels that should be closed byt the main loop
	// these channels cause transaction goroutines to return when closed.
	danglingFromClChannels           []chan string
	modifyDanglingFromClChannelsLock sync.Mutex
	//
	danglingClientCloseChannels           []chan struct{}
	modifyDanglingClientCloseChannelsLock sync.Mutex
}

func MakeClient(conn Conn) Client {
	return Client{
		publicKey: nil, // initially unset. When set, it implies the client has been added to the hub.

		conn:         conn,
		transactions: make(map[[IDLEN]byte]*transactionSocket),
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
		tSocket, exists := c.transactions[id]
		// if so, pass the message to that transaction
		if exists {
			select {
			case tSocket.fromCl <- string(msgBytes[IDLEN:]):
			default:
				c.writeTransactionMessage(tSocket.id, `{"error":"Buffer is occupied, message ignored"}`)
			}
			continue
		}

		// otherwise create a new transaction
		tNew := c.newTransaction(makeRoutine())
		tSocketNew := c.newTransactionSocket(tNew, id)

		// add to transaction list
		err = c.addTransactionSocket(tSocketNew)
		if err != nil {
			// client has disconnected
			// It's ok to close there here because nowhere else has access to them
			// and nowhere else will
			close(tSocketNew.fromCl)
			close(tSocketNew.clientCloseChan)
			close(tSocketNew.roChan)
			continue
		}

		// route routine (only one for a routine)
		go routeRoutine(hub, tNew)

		// route transaction (one for each client that interacts with this routine)
		go c.routeTransaction(tSocketNew)

		// send first message
		// read by routeTransaction
		tSocketNew.fromCl <- string(msgBytes[IDLEN:])
	}

	// breaks out here when the websocket is closed.
	c.close()

}

func (c *Client) newTransactionSocket(transaction *transaction, id [IDLEN]byte) *transactionSocket {
	roChan := make(chan RoutineOutput)
	return &transactionSocket{
		fromCl:          make(chan string, 1),
		clientCloseChan: make(chan struct{}),
		roChan:          roChan,
		transaction:     transaction,
		id:              id,
	}
}

func (c *Client) newTransaction(routine Routine) *transaction {
	return &transaction{
		pkToROChan: make(map[PublicKey](chan RoutineOutput)),
		riChan:     make(chan routineInputWrapper, RI_BUFFER_SIZE),
		routine:    routine,
	}
}

// Add a new transaction to the client.
// The channels in the transaction will be assosiated with the provided id in the client.
// Threadsafe.
func (c *Client) addTransactionSocket(t *transactionSocket) error {

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

			func() {
				// modify the transaction to add roChan
				defer t.transaction.pkToROChanLock.Unlock()
				t.transaction.pkToROChanLock.Lock()
				pk := c.GetPublicKey()
				if pk != nil {
					t.transaction.pkToROChan[*pk] = t.roChan
				}
				t.transaction.transactionSocketCount += 1
			}()

			c.transactions[t.id] = t
			return nil
		}
	}()
	return err
}

// threadsafe
func (c *Client) deleteTransactionSocket(id [IDLEN]byte) error {

	var ts *transactionSocket

	// delete the transaction, if it exists
	err := func() error {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()

		t0, exists := c.transactions[id]
		if !exists {
			return errors.New("transaction does not exist")
		}
		// save the socket so we can deal with the dangling channels
		ts = t0
		delete(c.transactions, id)
		return nil
	}()
	if err != nil {
		return err
	}

	// remove roChan from the inner transaction
	func() {
		defer ts.transaction.pkToROChanLock.Unlock()
		ts.transaction.pkToROChanLock.Lock()
		pk := c.publicKey
		if pk != nil {
			delete(ts.transaction.pkToROChan, *pk)
		}

		// if transaction has no more sockets (no clients can still communicate)
		// then close riChan - this causes routeRoutine to return, terminating its goroutine. This marks the end of the transaction
		ts.transaction.transactionSocketCount -= 1
		if ts.transaction.transactionSocketCount == 0 {
			close(ts.transaction.riChan)
		}
	}()

	// moves the channel to the dangling channels list.
	// mutex section above ensures that sinch we reach here, this is the only time that the specified transaction is being deleted
	// therefore we don't need to worry about duplicate channels ending up in the dangling channels list.
	func() {
		defer c.modifyDanglingFromClChannelsLock.Unlock()
		c.modifyDanglingFromClChannelsLock.Lock()
		c.danglingFromClChannels = append(c.danglingFromClChannels, ts.fromCl)
	}()
	func() {
		defer c.modifyDanglingClientCloseChannelsLock.Unlock()
		c.modifyDanglingClientCloseChannelsLock.Lock()
		c.danglingClientCloseChannels = append(c.danglingClientCloseChannels, ts.clientCloseChan)
	}()

	return nil
}

func (c *Client) close() {
	// set disconnected - prevent more transactions being added.
	func() {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()
		c.disconnected = true
	}()

	// delete all remaining transactions.
	// they might also try to delete themselves in their own goroutines,
	// but deleteTransactionSocket has synchronization to ensure that the transactions get deleted at most once.
	// also send a ClientClose message to the routines
	for _, t := range c.transactions {
		t.clientCloseChan <- struct{}{}
	}

	go func() {
		// close all dangling channels.
		// wait a bit for all routines to be deleted, and the channels added to this list
		time.Sleep(10 * time.Second)
		c.closeDanglingChannels()
	}()
}

// should run in a separate goroutine to the main Route() loop.
// this can ONLY be terminated by closing fromClMsg.
func (c *Client) routeTransaction(ts *transactionSocket) {

	// this function exits when ALL the below channels have closed.
	// both these channels are closed by the write end.
	roChanClosed := false
	fromClClosed := false
	clientCloseChanClosed := false
	allClosed := func() bool {
		return roChanClosed && fromClClosed && clientCloseChanClosed
	}

	ts.status = transactionStatus{
		done:         false,
		timeoutTimer: nil,
	}

	for {
		select {

		// output from routine
		case ro, ok := <-ts.roChan:

			if !ok {
				roChanClosed = true
				if allClosed() {
					return
				} else {
					continue
				}
			}

			// routine output received from another goroutine
			ts.status = c.processRoutineOutput(ts, ro)
			if ro.Done {
				c.deleteTransactionSocket(ts.id) // this also adds t.fromCl to the dangling channels list
			}

		// client close
		case _, ok := <-ts.clientCloseChan:
			if !ok {
				clientCloseChanClosed = true
				if allClosed() {
					return
				} else {
					continue
				}
			}

			if ts.status.done {
				continue
			}
			ts.status.done = true
			ts.transaction.riChan <- routineInputWrapper{
				args: RoutineInput{
					MsgType: RoutineMsgType_ClientClose,
					Pk:      c.GetPublicKey(),
					Msg:     "",
				},
				senderRoChan: ts.roChan,
			}
			c.deleteTransactionSocket(ts.id)

		// timeout
		case <-ts.status.timeoutTimer:

			if ts.status.done {
				continue
			}

			ts.transaction.riChan <- routineInputWrapper{
				args: RoutineInput{
					MsgType: RoutineMsgType_Timeout,
					Pk:      c.GetPublicKey(),
					Msg:     "",
				},
				senderRoChan: ts.roChan,
			}

		// message from client
		case fromClMsg, ok := <-ts.fromCl:

			if !ok {
				fromClClosed = true
				if allClosed() {
					return
				} else {
					continue
				}
			}

			if ts.status.done {
				// the routine has ended and the Transaction struct has been deleted,
				// but the main Route loop hasn't figured that out yet and is continuing to send us messages.
				// the next time Route gets to the top of its loop it should close fromClMsg.
				// ignore message, and keep waiting for fromClMsg to be closed.
				c.writeTransactionMessage(ts.id, `{"error":"transaction has terminated"}`)
				continue
			}

			ri := routineInputWrapper{
				args: RoutineInput{
					MsgType: RoutineMsgType_UsrMsg,
					Pk:      c.GetPublicKey(),
					Msg:     fromClMsg,
				},
				senderRoChan: ts.roChan,
			}
			// reject user messages if the buffer is occupied - prevent spam
			// should be very rare that any messages get rejected
			select {
			case ts.transaction.riChan <- ri:
			default:
				c.writeTransactionMessage(ts.id, `{"error":"buffer occupied"}`)
			}

		}

	}

}

// process RO for THIS client.
func (c *Client) processRoutineOutput(t *transactionSocket, ro RoutineOutput) transactionStatus {
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

// close leftover channels, causing routeTransaction() goroutines which use those channels to close
func (c *Client) closeDanglingChannels() {

	func() {
		defer c.modifyDanglingFromClChannelsLock.Unlock()
		c.modifyDanglingFromClChannelsLock.Lock()
		if len(c.danglingFromClChannels) > 0 {
			for _, ch := range c.danglingFromClChannels {
				close(ch)
			}
			c.danglingFromClChannels = make([]chan string, 0)
		}
	}()

	func() {
		defer c.modifyDanglingClientCloseChannelsLock.Unlock()
		c.modifyDanglingClientCloseChannelsLock.Lock()
		if len(c.danglingClientCloseChannels) > 0 {
			for _, ch := range c.danglingClientCloseChannels {
				close(ch)
			}
			c.danglingClientCloseChannels = make([]chan struct{}, 0)
		}
	}()
}

// thread safe & blocking.
func (c *Client) writeTransactionMessage(transactionID [IDLEN]byte, msg string) error {
	// concatenate transactionID and msg
	msgWithId := append(transactionID[:], []byte(msg)...)
	defer c.connWriteLock.Unlock()
	c.connWriteLock.Lock()
	return c.conn.WriteMessage(websocket.TextMessage, msgWithId)
}
