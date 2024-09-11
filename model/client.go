/**
 * Structure to keep track of client details.
 */

package model

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// length of key in bytes
const KEYLEN = 64

type PublicKey [KEYLEN]byte

// abstract function to pass all data for each instance to.
type InstanceHandler func(<-chan string, chan<- string, *Client)

type Client struct {
	// PRIVATE METHODS: not accessible outside current package
	publicKey *PublicKey
	// lock to prevent simultaneous writes to the websocket conn
	connWriteMessageMu sync.Mutex
	conn               *websocket.Conn
	// map of active transactions for this client; id -> transaction
	// should not access directly outside client.go
	transactions map[[IDLEN]byte]Transaction
}

func MakeClient(conn *websocket.Conn) Client {
	return Client{
		publicKey: nil, // initially unset

		conn:         conn,
		transactions: make(map[[IDLEN]byte]Transaction),
	}
}

func (c *Client) GetPublicKey() *PublicKey {
	return c.publicKey
}

func (c *Client) SetPublicKey(pk *PublicKey) {
	c.publicKey = pk
}

// a loop that demultiplexes messages and forwards them to correct handlers
func (c *Client) Route(handleInstance InstanceHandler) {

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
			// if t.FromCl is blocking (buffer is full) then reject the incoming message
			select {
			case t.FromCl <- string(msgBytes[IDLEN:]):
			default:
				t.ToCl <- "Message rejected - buffer occupied"
			}
			continue
		}

		// otherwise create a new transaction
		tNew := MakeTransactionWithId(id)

		// add to transaction list and add listeners
		c.AddTransaction(tNew)

		// start the new routine asyncronously
		go func() {
			defer c.DeleteTransaction(id)
			handleInstance(tNew.FromCl, tNew.ToCl, c)
			// routines.MasterRoutine(tNew.FromCl, tNew.ToCl, c)
		}()

		// send the initiating message to the routine
		// it should be waiting.
		tNew.FromCl <- string(msgBytes[IDLEN:])

	}
}

// Add a new transaction to the client.
// The channels in the transaction will be assosiated with the provided id in the client.
func (c *Client) AddTransaction(t Transaction) {

	_, idExists := c.transactions[t.Id]
	if idExists {
		panic("Attempted to registed a transaction id that already exists!")
	}

	c.transactions[t.Id] = t

	// concurrent function to forward messages on ToCl to the websocket
	go func() {
		for msg := range t.ToCl {
			// prepend the transaction id
			msgWithId := append(t.Id[:], []byte(msg)...)

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

func (c *Client) DeleteTransaction(id [IDLEN]byte) {

	// might have to wait a bit for the last message to be sent.
	// dunno I saw that online
	// time.Sleep(time.Second)

	t, exists := c.transactions[id]
	if !exists {
		panic("Attempted to deregister a transaction id that does not exist.")
	}

	close(t.FromCl)
	close(t.ToCl)
	delete(c.transactions, id)
}
