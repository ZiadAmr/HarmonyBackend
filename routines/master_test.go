package routines

import (
	"harmony/backend/model"
	"testing"
	"time"
)

// ==================================================================

// fake `Routines` implementation that tracks the names of the subroutine methods that were called
type FakeRoutinesCallTracker struct {
	calls []string
}

func (r *FakeRoutinesCallTracker) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string) {
	r.calls = append(r.calls, "ComeOnline")
}
func (r *FakeRoutinesCallTracker) EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string) {
	r.calls = append(r.calls, "EstablishConnectionToPeer")
}

//===================================================================

// another fake `routines` implementation used to mock the routines for various other behaviours
type FakeRoutines struct {
	// routine signals that it has been called
	called chan struct{}
	// tell the routine to return
	done chan struct{}

	client *model.Client
	hub    *model.Hub
	fromCl chan string
	toCl   chan string
}

// grab the args so we can check what the function was called with.
func (r *FakeRoutines) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string) {

	r.client = client
	r.hub = hub
	r.fromCl = fromCl
	r.toCl = toCl

	// signal that this function has been called
	r.called <- struct{}{}

	// simulate the routine running - don't return immediately, wait until told.
	<-r.done
}

// timeout
func (r *FakeRoutines) EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string) {
	time.Sleep(1000 * time.Second)
}

//===================================================================

func TestMasterRoutine(t *testing.T) {

	t.Run("Master routine calls no routines and returns error when schema does not match", func(t *testing.T) {

		invalidMessages := []string{
			`{"initiate": "thisIsARoutineThatDoesNotExist"}`,
			`{}`,
			`this is not valid json`,
		}

		mockClient := &model.Client{}
		mockHub := model.NewHub()

		for _, tt := range invalidMessages {
			t.Run(tt, func(t *testing.T) {
				mockTransaction := model.MakeTransaction()
				fakeRoutines := FakeRoutinesCallTracker{}

				// send non-matching json
				mockTransaction.FromCl <- tt

				done := make(chan struct{})
				// run function
				go func() {
					masterRoutine(&fakeRoutines, mockClient, mockHub, mockTransaction.FromCl, mockTransaction.ToCl)
					done <- struct{}{}
				}()

				select {
				case <-shortTimePassed():
					t.Errorf("Expected an error message")
				case msg := <-mockTransaction.ToCl:
					if !isErrorMessage(msg) {
						t.Errorf("Expected an error message. Got %s", msg)
					}
				}

				select {
				case <-shortTimePassed():
					t.Errorf("Timeout waiting for master function to return")
				case <-done:
				}

				totalCount := len(fakeRoutines.calls)

				if totalCount != 0 {
					t.Errorf("Total routine call count: expected %v got %v", 0, totalCount)
				}
			})
		}

	})

	t.Run("Master routine calls correct routine", func(t *testing.T) {

		tests := []struct {
			initiateKeyword     string
			routineFunctionName string
		}{
			{"comeOnline", "ComeOnline"},
			{"establishConnectionToPeer", "EstablishConnectionToPeer"},
		}

		for _, tt := range tests {
			t.Run(tt.initiateKeyword, func(t *testing.T) {
				mockClient := &model.Client{}
				mockHub := model.NewHub()

				mockTransaction := model.MakeTransaction()
				fakeRoutines := &FakeRoutinesCallTracker{}

				// send matching schema
				mockTransaction.FromCl <- `{
					"initiate": "` + tt.initiateKeyword + `"
				}`
				// run function
				masterRoutine(fakeRoutines, mockClient, mockHub, mockTransaction.FromCl, mockTransaction.ToCl)

				// check only the correct routines was called
				thisRoutineCount := countOccurrences(fakeRoutines.calls, tt.routineFunctionName)
				totalCount := len(fakeRoutines.calls)

				if thisRoutineCount != 1 {
					t.Errorf("Call count: expected %v got %v", 1, thisRoutineCount)
				}
				if totalCount != 1 {
					t.Errorf("Total routine call count: expected %v got %v", 1, totalCount)
				}
			})
		}

	})

	t.Run("Master routine passes all user messages to handlers", func(t *testing.T) {

		/*
			In this test the ComeOnline routine handler is being mocked.
			Instead of its normal behaviour it does the following:
			1. Copy all its arguments (including fromCl) to a struct accessible from this test
			2. Signal that it has been called
			3. Wait for a signal before it returns.

			The main steps of this test:
			1. Send a sequence of messages to the master routine, the first of which should invoke ComeOnline
			2. Once ComeOnline has been invoked, check that each message sent to ComeOnline's fromCl argument matches the message we're sending to the master routine
			3. Tell ComeOnline to return.

			The main point of this test is to ensure that the first {"initiate":"comeOnline"} message is being passed through to ComeOnline, since the master routine has to read and parse this message first before it can invoke ComeOnline.
		*/

		test := []string{
			`{"initiate":"comeOnline"}`,
			"message 2",
			"message 3",
		}

		mockClient := &model.Client{}
		mockHub := model.NewHub()

		// struct with the mock ComeOnline method defined on it
		r := &FakeRoutines{
			// message sent to this channel when fake `ComeOnline` invoked
			called: make(chan struct{}),
			// send a message to this channel to make ComeOnline return.
			done: make(chan struct{}),
		}
		defer close(r.called)
		defer close(r.done)

		fromCl := make(chan string)

		// don't need toCl here. just for mocking
		toCl := make(chan string)
		defer close(toCl)

		// to check that the child goroutines have finished
		done0 := make(chan struct{}, 1)
		done1 := make(chan struct{}, 1)
		defer close(done0)
		defer close(done1)

		// run master routine
		go func() {
			defer close(fromCl)
			masterRoutine(r, mockClient, mockHub, fromCl, toCl)

			// signal that this goroutine has completed
			done0 <- struct{}{}
		}()

		// observe and verify what is being sent to the ComeOnline mock
		go func() {

			// wait for ComeOnline to be invoked
			<-r.called

			// compare each message send to ComeOnline to the expected.
			// count the messages so that we know when to make ComeOnline return
		loop:
			for i := 0; i < len(test); i++ {

				expect := test[i]

				select {
				case <-shortTimePassed():
					t.Errorf("Timeout waiting for message %d/%d: %s", i+1, len(test), expect)
					break loop
				case got := <-r.fromCl:
					if expect != got {
						t.Errorf("Unexpected message sent to routine. Expected %s got %s", expect, got)
					}
				}
			}

			// tell ComeOnline mock to return
			r.done <- struct{}{}

			// signal that this goroutine has completed
			done1 <- struct{}{}

		}()

		// send messages to the master routine
		for _, stepStr := range test {
			select {
			case fromCl <- stepStr:
			case <-shortTimePassed():
				t.Errorf("Timeout")
				return
			}
		}

		// wait for goroutines to finish
		<-done0
		<-done1

	})

	// t.Run("Master routine kills routines if user sends terminate:cancel property", func(t *testing.T) {
	// 	// TODO
	// })

}
