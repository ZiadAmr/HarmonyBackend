package model

import "fmt"

type routineInputWrapper struct {
	args         RoutineInput
	senderRoChan chan RoutineOutput
}

func routeRoutine(hub *Hub, t *transaction) {

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
		distributeRoutineOutputs(hub, t, &closedRoChans, riw.senderRoChan, ros)

		if riw.args.MsgType == RoutineMsgType_ClientClose {
			closedRoChans[riw.senderRoChan] = struct{}{}
			close(riw.senderRoChan)
		}

	}

}

// send routine outputs to correct clients.
func distributeRoutineOutputs(hub *Hub, t *transaction, closedRoChans *map[chan RoutineOutput]struct{}, senderRoChan chan RoutineOutput, ros []RoutineOutput) {

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
					go peerClient.routeTransaction(tSocket)
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
