/**
 * Structure to keep track of client details.
 */

package model

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// routine input buffer size
const RI_BUFFER_SIZE = 10

// maximum number of concurrent transactions created by this client
const MAX_TRANSACTIONS = 1

type PublicKey string

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
	// map of active transactionSockets for this client; id -> transactionSocket
	// should not access directly outside client.go
	transactionSockets     map[[IDLEN]byte]*transactionSocket
	modifyTransactionsLock sync.Mutex
	// used to prevent new transactions being added after broken out of the main loop
	// must use modifyTransactionsLock when reading or editing
	disconnected bool
	// channels that should be closed byt the main loop
	// these channels cause transaction goroutines to return when closed.
	danglingClientMsgChannels           []chan string
	modifyDanglingClientMsgChannelsLock sync.Mutex
	//
	danglingClientCloseChannels           []chan struct{}
	modifyDanglingClientCloseChannelsLock sync.Mutex

	transactionCount       int
	modifyTransactionCount sync.Mutex

	// PUBLIC METHODS
	// lock to prevent simultaneous comeOnline transactions
	ComeOnlineLock sync.Mutex
}

func MakeClient(conn Conn) Client {
	return Client{
		publicKey: nil, // initially unset. When set, it implies the client has been added to the hub.

		conn:               conn,
		transactionSockets: make(map[[IDLEN]byte]*transactionSocket),
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
		tSocket, exists := c.transactionSockets[id]
		// if so, pass the message to that transaction
		if exists {
			select {
			case tSocket.clientMsgChan <- string(msgBytes[IDLEN:]):
			default:
				c.writeTransactionMessage(tSocket.id, `{"error":"Buffer is occupied, message ignored"}`)
			}
			continue
		}

		// otherwise need to create a new transaction

		// limit number of transactions - check
		if c.transactionCount >= MAX_TRANSACTIONS {
			c.writeTransactionMessage(id, `{"terminate":"cancel","error":"Max number of transactions (`+strconv.Itoa(MAX_TRANSACTIONS)+`) reached, message ignored"}`)
			continue
		}
		func() {
			defer c.modifyTransactionCount.Unlock()
			c.modifyTransactionCount.Lock()
			c.transactionCount++
		}()

		tNew := newTransaction(makeRoutine())
		tSocketNew := newTransactionSocket(tNew, id)

		// add to transaction list
		err = c.addTransactionSocket(tSocketNew)
		if err != nil {
			// client has disconnected
			// It's ok to close there here because nowhere else has access to them
			// and nowhere else will
			close(tSocketNew.clientMsgChan)
			close(tSocketNew.clientCloseChan)
			close(tSocketNew.roChan)
			continue
		}

		// route transaction
		go func() {
			tNew.route(hub)
			func() {
				defer c.modifyTransactionCount.Unlock()
				c.modifyTransactionCount.Lock()
				c.transactionCount--
			}()
		}()

		// route transaction socket (one for each client that interacts with this routine)
		go c.routeTransactionSocket(tSocketNew)

		// send first message
		// read by routeTransactionSocket
		tSocketNew.clientMsgChan <- string(msgBytes[IDLEN:])
	}

	// breaks out here when the websocket is closed.
	c.close()

}

// Add a new transaction to the client.
// The channels in the transaction will be assosiated with the provided id in the client.
// Threadsafe.
func (c *Client) addTransactionSocket(t *transactionSocket) error {

	_, idExists := c.transactionSockets[t.id]
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

			c.transactionSockets[t.id] = t
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

		t0, exists := c.transactionSockets[id]
		if !exists {
			return errors.New("transaction does not exist")
		}
		// save the socket so we can deal with the dangling channels
		ts = t0
		delete(c.transactionSockets, id)
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
		defer c.modifyDanglingClientMsgChannelsLock.Unlock()
		c.modifyDanglingClientMsgChannelsLock.Lock()
		c.danglingClientMsgChannels = append(c.danglingClientMsgChannels, ts.clientMsgChan)
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
	for _, t := range c.transactionSockets {
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
func (c *Client) routeTransactionSocket(ts *transactionSocket) {

	// this function exits when ALL the below channels have closed.
	// both these channels are closed by the write end.
	roChanClosed := false
	clientMsgChanClosed := false
	clientCloseChanClosed := false
	allClosed := func() bool {
		return roChanClosed && clientMsgChanClosed && clientCloseChanClosed
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
				ts.roChan = nil
				if allClosed() {
					return
				} else {
					continue
				}
			}

			// routine output received from another goroutine
			ts.status = c.processRoutineOutput(ts, ro)
			if ro.Done {
				c.deleteTransactionSocket(ts.id) // this also adds t.clientMsgChan to the dangling channels list
			}

		// client close
		case _, ok := <-ts.clientCloseChan:
			if !ok {
				clientCloseChanClosed = true
				ts.clientCloseChan = nil
				if allClosed() {
					return
				} else {
					continue
				}
			}

			if ts.status.done {
				continue
			}
			riw := routineInputWrapper{
				args: RoutineInput{
					MsgType: RoutineMsgType_ClientClose,
					Pk:      c.GetPublicKey(),
					Msg:     "",
				},
				senderRoChan: ts.roChan,
			}

			select {
			// try to send. might be blocked
			case ts.transaction.riChan <- riw:
			default:
				// keep trying to send riw while listening and processing roChan at the same time
				// this ensures that the route transaction goroutine won't be blocked if it tries to send a ro to us - riChan buffer can empty so that we can eventually send the riw
				roChanWasClosedDuringThis := c.sendMessageAndAvoidRoChanDeadlock(riw, ts)
				if roChanWasClosedDuringThis {

					roChanClosed = true
				}
			}
			ts.status.done = true
			c.deleteTransactionSocket(ts.id)

		// timeout
		case <-ts.status.timeoutTimer:

			ts.status.timeoutTimer = nil

			if ts.status.done {
				continue
			}

			riw := routineInputWrapper{
				args: RoutineInput{
					MsgType: RoutineMsgType_Timeout,
					Pk:      c.GetPublicKey(),
					Msg:     "",
				},
				senderRoChan: ts.roChan,
			}

			select {
			// try to send. might be blocked
			case ts.transaction.riChan <- riw:
			default:
				// keep trying to send riw while listening and processing roChan at the same time
				roChanWasClosedDuringThis := c.sendMessageAndAvoidRoChanDeadlock(riw, ts)
				if roChanWasClosedDuringThis {
					roChanClosed = true
				}
			}

		// message from client
		case msg, ok := <-ts.clientMsgChan:

			if !ok {
				ts.clientMsgChan = nil
				clientMsgChanClosed = true
				if allClosed() {
					return
				} else {
					continue
				}
			}

			if ts.status.done {
				// the routine has ended and the Transaction struct has been deleted,
				// but the main Route loop hasn't figured that out yet and is continuing to send us messages.
				// the next time Route gets to the top of its loop it should close clientMsgChan.
				// ignore message, and keep waiting for clientMsgChan to be closed.
				c.writeTransactionMessage(ts.id, `{"error":"transaction has terminated"}`)
				continue
			}

			ri := routineInputWrapper{
				args: RoutineInput{
					MsgType: RoutineMsgType_UsrMsg,
					Pk:      c.GetPublicKey(),
					Msg:     msg,
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

// some messages (client close and timeout) we must send this message to the routine - we can't throw them away if the buffer is full
// otherwise the routine might never terminate properly
// but also we can't block this goroutine by trying to write to riChan, because this could cause a deadlock if the route transaction goroutine tries to send a routine output to us.
// solution - do both at once.
// Returns true if roChan was closed during this loop.
func (c *Client) sendMessageAndAvoidRoChanDeadlock(riw routineInputWrapper, ts *transactionSocket) bool {

	roChanClosed := false

AntiDeadlockLoop:
	for {
		if ts.status.done {
			break AntiDeadlockLoop
		}
		select {
		// try to send the message
		case ts.transaction.riChan <- riw:
			break AntiDeadlockLoop
		// if blocked we can read from rochan to allow the richan buffer to empty
		case ro, ok := <-ts.roChan:
			if !ok {
				ts.roChan = nil
				roChanClosed = true
				continue
			}
			// do usual processing stuff with the ro
			ts.status = c.processRoutineOutput(ts, ro)
			if ro.Done {
				c.deleteTransactionSocket(ts.id)
			}
		}
	}

	return roChanClosed

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

// close leftover channels, causing routeTransactionSocket() goroutines which use those channels to close
func (c *Client) closeDanglingChannels() {

	func() {
		defer c.modifyDanglingClientMsgChannelsLock.Unlock()
		c.modifyDanglingClientMsgChannelsLock.Lock()
		if len(c.danglingClientMsgChannels) > 0 {
			for _, ch := range c.danglingClientMsgChannels {
				close(ch)
			}
			c.danglingClientMsgChannels = make([]chan string, 0)
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
