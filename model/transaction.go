package model

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/exp/rand"
)

const IDLEN = 16

type transactionStatus struct {
	done         bool
	timeoutTimer <-chan time.Time
}

// each client interacting with a given transaction has one of these
type transactionSocket struct {
	// messages from client
	clientMsgChan chan string
	// message is sent if the client disconnects
	clientCloseChan chan struct{}
	// routine outputs to the sent to the client
	roChan chan RoutineOutput
	// transaction (socket) id
	id [IDLEN]byte

	transaction *transaction
	status      transactionStatus
}

type routineInputWrapper struct {
	args         RoutineInput
	senderRoChan chan RoutineOutput
}

// instance of a routine
type transaction struct {
	// routine output channels - for communication between users
	pkToROChan map[PublicKey](chan RoutineOutput)
	// also requires pkToROChanLock
	transactionSocketCount int
	pkToROChanLock         sync.Mutex

	routine Routine

	// wrappers around inputs for the .Next() method of the routine
	riChan chan routineInputWrapper
}

func (t *transaction) route(hub *Hub) {

	// within this function and subfunctions is the only place where roChans can be closed.
	// this ensures that the routine always explicity ends a client socket (by sending Done=true in a RoutineOutput), or that the routine is aware when the client disconnects.
	// Therefore the routine can be programmed to never send messages to clients with closed roChans.

	// set of closed roChans, so we can ignore any messages from the owner of these.
	closedRoChans := make(map[chan RoutineOutput]struct{})

	// breaks out when the riChan is closed
	// this occurs when the last client
	for riw := range t.riChan {

		// ignore messages from clients with closed routine output channels
		// (the transaction with this client has been terminated)
		_, isClosed := closedRoChans[riw.senderRoChan]
		if isClosed {
			continue
		}

		ros := t.routine.Next(riw.args)
		t.distributeRoutineOutputs(hub, &closedRoChans, riw.senderRoChan, ros)

		if riw.args.MsgType == RoutineMsgType_ClientClose {
			closedRoChans[riw.senderRoChan] = struct{}{}
			close(riw.senderRoChan)
		}

	}

}

// send routine outputs to correct clients.
func (t *transaction) distributeRoutineOutputs(hub *Hub, closedRoChans *map[chan RoutineOutput]struct{}, senderRoChan chan RoutineOutput, ros []RoutineOutput) {

	for _, routineOutput := range ros {
		if routineOutput.Pk == nil {
			senderRoChan <- routineOutput
			if routineOutput.Done {
				(*closedRoChans)[senderRoChan] = struct{}{}
				close(senderRoChan)
			}
		} else {
			roChan, exists := t.pkToROChan[*routineOutput.Pk]
			if exists {
				roChan <- routineOutput
				if routineOutput.Done {
					(*closedRoChans)[senderRoChan] = struct{}{}
					close(senderRoChan)
				}
			} else {
				// todo need to create a new transaction if it does not exist
				peerClient, exists := hub.GetClient(*routineOutput.Pk)
				if !exists {
					fmt.Printf("client does not exist")
					continue
				}
				tSocket := peerClient.newTransactionSocket(t, newId())
				err := peerClient.addTransactionSocket(tSocket)
				if err == nil {
					go peerClient.routeTransactionSocket(tSocket)
					tSocket.roChan <- routineOutput
					if routineOutput.Done {
						(*closedRoChans)[roChan] = struct{}{}
						close(roChan)
					}
				}
			}

		}
	}
}

// genreate a random transaction id
func newId() [IDLEN]byte {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	var seededRand *rand.Rand = rand.New(rand.NewSource(uint64(time.Now().UnixNano())))

	var id [IDLEN]byte
	for i := range id {
		id[i] = charset[seededRand.Intn(len(charset))]
	}
	return id

}
