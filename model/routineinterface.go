package model

import "time"

type RoutineOutput struct {
	// 0 or more messages to send to the client
	Msgs []string
	// whether the routine has completed and should no longer accept new messages
	Done bool
	// max time to wait for next user input.
	// currently unused
	TimeoutDuration time.Duration
	TimeoutEnabled  bool
}

func MakeRoutineOutput(done bool, msgs ...string) RoutineOutput {
	return RoutineOutput{
		Msgs:            msgs,
		Done:            done,
		TimeoutEnabled:  false,
		TimeoutDuration: 0,
	}
}

type Routine interface {
	// called on each message from the client
	Next(string) RoutineOutput
}
