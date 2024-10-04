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

// abstract function to pass all data for each instance to.
type InstanceHandler func(fromCl chan string, toCl chan string)

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
	transactions           map[[IDLEN]byte]Transaction
	modifyTransactionsLock sync.Mutex
	// channels that should be closed byt the main loop
	// these channels cause transaction goroutines to return when closed.
	danglingChannels           []chan string
	modifyDanglingChannelsLock sync.Mutex
}

func MakeClient(conn Conn) Client {
	return Client{
		publicKey: nil, // initially unset. When set, it implies the client has been added to the hub.

		conn:         conn,
		transactions: make(map[[IDLEN]byte]Transaction),
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
func (c *Client) Route(masterRoutineConstructor func() Routine) {

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
		t, exists := c.transactions[id]
		// if so, pass the message to that transaction
		if exists {
			select {
			case t.fromCl <- string(msgBytes[IDLEN:]):
			default:
				c.writeTransactionMessage(t.Id, `{"error":"Buffer is occupied, message ignored"}`)
			}
			continue
		}

		// otherwise create a new transaction
		tNew := Transaction{
			fromCl:  make(chan string, 1),
			Id:      id,
			Routine: masterRoutineConstructor(),
		}

		// add to transaction list and add listeners
		c.addTransaction(tNew)

		tNew.fromCl <- string(msgBytes[IDLEN:])

		// route
		go c.routeTransaction(tNew)

	}

	// breaks out here when the websocket is closed.
	c.close()

}

// Add a new transaction to the client.
// The channels in the transaction will be assosiated with the provided id in the client.
// Threadsafe.
func (c *Client) addTransaction(t Transaction) {

	_, idExists := c.transactions[t.Id]
	if idExists {
		panic("Attempted to registed a transaction id that already exists!")
	}

	func() {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()
		c.transactions[t.Id] = t
	}()
}

// threadsafe
func (c *Client) deleteTransaction(id [IDLEN]byte) error {

	var t Transaction

	// delete the transaction, if it exists
	err := func() error {
		defer c.modifyTransactionsLock.Unlock()
		c.modifyTransactionsLock.Lock()

		t0, exists := c.transactions[id]
		if !exists {
			return errors.New("Transaction does not exist")
		}
		t = t0
		delete(c.transactions, id)
		return nil
	}()
	if err != nil {
		return err
	}

	// moves the channel to the dangling channels list.
	// mutex section above ensures that sinch we reach here, this is the only time that the specified transaction is being deleted
	// therefore we don't need to worry about duplicate channels ending up in the dangling channels list.
	func() {
		defer c.modifyDanglingChannelsLock.Unlock()
		c.modifyDanglingChannelsLock.Lock()
		c.danglingChannels = append(c.danglingChannels, t.fromCl)

	}()

	return nil
}

func (c *Client) close() {
	// delete all remaining transactions.
	// they might also try to delete themselves in their own goroutines,
	// but DeleteTransaction has synchronization to ensure that the transactions get deleted at most once.
	for tid := range c.transactions {
		c.deleteTransaction(tid)
	}
	c.closeDanglingChannels()
}

// should run in a separate goroutine to the main Route() loop.
// this can ONLY be terminated by closing fromClMsg.
func (c *Client) routeTransaction(t Transaction) {

	done := false
	var timeoutTimer <-chan time.Time

	for {
		select {
		case <-timeoutTimer:
			c.writeTransactionMessage(t.Id, `{"terminate":"cancel","error":"timeout"}`)
			done = true
			c.deleteTransaction(t.Id)

		case fromClMsg, ok := <-t.fromCl:
			if !ok {
				// terminate this fn only when fromCl closes.
				return
			}
			if done {
				// the routine has ended and the Transaction struct has been deleted,
				// but the main Route loop hasn't figured that out yet and is continuing to send us messages.
				// the next time Route gets to the top of its loop it should close fromClMsg.
				// ignore message, and keep waiting for fromClMsg to be closed.
				c.writeTransactionMessage(t.Id, `{"error":"transaction has terminated"}`)
				continue
			}

			stepOutput := t.Routine.Next(fromClMsg)
			for _, toClMsg := range stepOutput.Msgs {
				// write message
				err := c.writeTransactionMessage(t.Id, toClMsg)
				if err != nil {
					fmt.Printf("Error writing message: " + err.Error())
				}
			}

			// set the timeout
			if stepOutput.TimeoutEnabled {
				timeoutTimer = time.After(stepOutput.TimeoutDuration)
			}

			done = stepOutput.Done
			if stepOutput.Done {
				c.deleteTransaction(t.Id) // this also adds t.fromCl to the dangling channels list
			}
		}

	}

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
