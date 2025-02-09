package routines

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"harmony/backend/model"
	"time"
)

type ChatRoutineDemo struct {
	client *model.Client
	hub    *model.Hub
}

func NewChatRoutineDemo(client *model.Client, hub *model.Hub) model.Routine {
	return &ChatRoutineDemo{
		client: client,
		hub:    hub,
	}
}

func (r *ChatRoutineDemo) Next(args model.RoutineInput) []model.RoutineOutput {
	switch args.MsgType {
	case model.RoutineMsgType_Timeout:
		return []model.RoutineOutput{{
			Msgs: []string{`Timed out waiting for a response - you no longer have access to this transaction. Start a new one if you want to send additional messages`},
			Done: true,
		}}
	case model.RoutineMsgType_UsrMsg:

		usrMsg := struct {
			PublicKey string
			Msg       string
		}{}

		json.Unmarshal([]byte(args.Msg), &usrMsg)
		peerPkBytes, _ := hex.DecodeString(usrMsg.PublicKey)
		peerPk := (*model.PublicKey)(peerPkBytes)

		// the first message that they send sets their own public key and doesn't actually send any message
		// kinda hacky, but this is a demo so who cares
		myPk := r.client.GetPublicKey()
		if myPk == nil {
			r.client.SetPublicKey(peerPk)
			r.hub.AddClient(*peerPk, r.client)
			return []model.RoutineOutput{{
				Pk:              nil, // client sending the message
				Msgs:            []string{"Your public key has been set."},
				Done:            true, // yeet the transaction out of the windnow
				TimeoutEnabled:  true,
				TimeoutDuration: 60 * time.Second,
			}}
		}

		return []model.RoutineOutput{
			{
				Pk:              peerPk, // the peer
				Msgs:            []string{usrMsg.Msg},
				Done:            false, // keep transaction alive
				TimeoutEnabled:  true,
				TimeoutDuration: 60 * time.Second,
			},
			{
				Pk:              nil,   // the client sending the message
				Done:            false, // keep transaction alive
				TimeoutEnabled:  true,
				TimeoutDuration: 60 * time.Second,
			},
		}

	case model.RoutineMsgType_ClientClose:
		fmt.Printf("Client has disconnected\n")
		return []model.RoutineOutput{}

	default:
		panic("somehow we got a message type that wasn't even in the enum??")
	}
}
