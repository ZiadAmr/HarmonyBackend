/**
 * Structure to keep track of client details.
 */

package main

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	// lock to prevent simultaneous writes to the conn
	connWriteMessageMu sync.Mutex

	conn      *websocket.Conn
	publicKey string

	// should not access directly outside client.go
	transactions map[[IDLEN]byte]Transaction
}

func makeClient(conn *websocket.Conn) Client {
	return Client{
		conn:      conn,
		publicKey: "", // initially unset
		// map of active transactions for this client; id -> transaction
		transactions: make(map[[IDLEN]byte]Transaction),
	}
}

// a loop that demultiplexes messages and forwards them to correct handlers
func (c *Client) route() {

	for {
		// read from websocket (blocking)
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message: " + err.Error())
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
			t.fromCl <- string(msgBytes[IDLEN:])
			continue
		}

		// otherwise create a new transaction
		tNew := makeTransactionWithId(id)

		// add to transaction list and add listeners
		c.registerTransaction(tNew)

		// start the new routine asyncronously
		go func() {
			defer c.deregisterTransaction(id)
			MasterRoutine(tNew.fromCl, tNew.toCl, c)
		}()

		// send the initiating message to the routine
		// it should be waiting.
		tNew.fromCl <- string(msgBytes[IDLEN:])

	}
}

func (c *Client) registerTransaction(transaction Transaction) {

	_, idExists := c.transactions[transaction.id]
	if idExists {
		panic("Attempted to registed a transaction id that already exists!")
	}

	c.transactions[transaction.id] = transaction

	// concurrent function to forward messages sent to toCl
	go func() {
		for msg := range transaction.toCl {
			// prepend the transaction id
			msgWithId := append(transaction.id[:], []byte(msg)...)

			// write message
			err := func() error {
				// lock with mutex to prevent multiple messages being sent at once
				defer c.connWriteMessageMu.Unlock()
				c.connWriteMessageMu.Lock()
				return c.conn.WriteMessage(websocket.TextMessage, msgWithId)
			}()

			if err != nil {
				fmt.Println("Error writing message: " + err.Error())
				break
			}

		}
	}()

	// transaction messages sent to the websocket are forwarded automatically if c.route has been called.

}

func (c *Client) deregisterTransaction(id [IDLEN]byte) {

	// might have to wait a bit for the last message to be sent.
	// dunno I saw that online
	// time.Sleep(time.Second)

	t, exists := c.transactions[id]
	if !exists {
		panic("Attempted to deregister a transaction id that does not exist.")
	}

	close(t.fromCl)
	close(t.toCl)
	delete(c.transactions, id)
}
