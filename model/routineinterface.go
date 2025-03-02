package model

import "time"

type Routine interface {
	// called on each message from the client.
	Next(args RoutineInput) []RoutineOutput
}

type RoutineInput struct {
	MsgType RoutineMsgType
	// public key is nil if unset.
	Pk *PublicKey
	// can be ignored if MsgType is not RoutineMsgType_UsrMsg.
	Msg string
}

type RoutineMsgType int

const ( // enum
	RoutineMsgType_UsrMsg RoutineMsgType = iota
	RoutineMsgType_Timeout
	RoutineMsgType_ClientClose
)

type RoutineOutput struct {
	// Public key of the client to send messages to.
	// Nil to reply to the client that sent the message.
	Pk *PublicKey
	// 0 or more messages to send to the client
	Msgs []string
	// whether the routine should no longer accept messages from the client.
	// routine should NOT send any more messages after sending Done=true, or receiving a msg of msgType RoutineMsgType_ClientClose. This could result in a panic().
	Done bool
	// if no message is received within the timeout then the routine gets a .Next() with message type RoutineMsgType_Timeout
	// and can deal with it however it wants (e.g. by returning a RoutineOutput with done=true)
	TimeoutDuration time.Duration
	TimeoutEnabled  bool
}

// you don't need to use this - you can just create the struct directly
func MakeRoutineOutput(done bool, msgs ...string) RoutineOutput {
	return RoutineOutput{
		Pk:              nil,
		Msgs:            msgs,
		Done:            done,
		TimeoutEnabled:  false,
		TimeoutDuration: 0,
	}
}
