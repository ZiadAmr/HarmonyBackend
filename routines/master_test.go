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

func (r *FakeRoutinesCallTracker) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string) {
	r.calls = append(r.calls, "ComeOnline")
}
func (r *FakeRoutinesCallTracker) EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string) {
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
	errCl  chan string
}

// grab the args so we can check what the function was called with.
func (r *FakeRoutines) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string) {

	r.client = client
	r.hub = hub
	r.fromCl = fromCl
	r.toCl = toCl
	r.errCl = errCl

	// signal that this function has been called
	r.called <- struct{}{}

	// simulate the routine running - don't return immediately, wait until told.
	<-r.done
}

// timeout
func (r *FakeRoutines) EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string) {
	time.Sleep(1000 * time.Second)
}

//===================================================================

// count number of occurrences of an element in a slice
func countOccurrences[K comparable](slice []K, el K) int {
	count := 0
	for _, item := range slice {
		if item == el {
			count++
		}
	}
	return count
}

func TestMasterRoutine(t *testing.T) {

	t.Run("Master routine calls no routines when schema does not match", func(t *testing.T) {

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

				// run function
				masterRoutine(&fakeRoutines, mockClient, mockHub, mockTransaction.FromCl, mockTransaction.ToCl)

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
			done0 <- struct{}{}
		}()

		// observe and verify what is being sent to the ComeOnline mock
		go func() {

			// wait for ComeOnline to be invoked
			<-r.called
			// check all messages were passed to it
			currentStep := 0
			for msg := range r.fromCl { // loop ends when r.fromCl is closed by other goroutine.
				expected := test[currentStep]
				got := msg
				if expected != got {
					t.Errorf("Unexpected message sent to routine. Expected %s got %s", expected, got)
				}
				currentStep++
			}
			// check all messages received when channel closes
			expected := len(test)
			got := currentStep
			if expected != got {
				t.Errorf("Wrong number of messages sent to routine. Expected %d got %d", expected, got)
			}

			done1 <- struct{}{}

		}()

		// send messages to the routine
		for _, stepStr := range test {
			select {
			case fromCl <- stepStr:
			case <-shortTimePassed():
				t.Errorf("Timeout")
				return
			}
		}
		// tell ComeOnline to return, we've finished sending messages now.
		// it sometimes takes some time for the last message to arrive at the ComeOnline routine, so just wait a bit before telling the routine to return
		time.Sleep(1 * time.Millisecond)
		r.done <- struct{}{}

		// wait for goroutines to finish
		<-done0
		<-done1

	})

	// t.Run("Master routine kills routines if they hang for too long", func(t *testing.T) {
	// 	mockClient := &model.Client{}
	// 	mockHub := model.NewHub()

	// 	mockTransaction := model.MakeTransaction()
	// 	// this implementation of comeonline routine just hangs for a while.
	// 	fakeRoutines := &FakeRoutines{}

	// 	// inject this value for routine timeouts instead of the actual value
	// 	timeout := 10 * time.Millisecond

	// 	done := make(chan struct{})

	// 	go func() {
	// 		masterRoutine(fakeRoutines, timeout, mockClient, mockHub, mockTransaction.FromCl, mockTransaction.ToCl)
	// 		done <- struct{}{}
	// 	}()

	// 	// initiate the fake comeOnline hanging routine
	// 	mockTransaction.FromCl <- `{
	// 		"initiate": "establishConnectionToPeer"
	// 	}`

	// 	// expect a terminate:cancel to be sent from the master routine.

	// 	var output string
	// 	select {
	// 	case <-time.After(timeout * 2):
	// 		t.Errorf("No message sent from master routine")
	// 		return
	// 	case output = <-mockTransaction.ToCl:
	// 	}

	// 	outputLoader := gojsonschema.NewStringLoader(output)
	// 	schemaLoader := gojsonschema.NewStringLoader(`{
	// 		"$schema": "https://json-schema.org/draft/2020-12/schema",
	// 		"type": "object",
	// 		"properties": {
	// 			"terminate": {
	// 			"const":"cancel",
	// 			}
	// 		},
	// 		"required": ["terminate"],
	// 	}`)

	// 	result, err := gojsonschema.Validate(schemaLoader, outputLoader)

	// 	if err != nil {
	// 		t.Errorf(err.Error())
	// 		return
	// 	}
	// 	if !result.Valid() {
	// 		t.Errorf("output did not match schema. Output: %s", output)
	// 	}

	// 	// check that the routine ended
	// 	select {
	// 	case <-time.After(timeout * 2):
	// 		t.Errorf("master routine didn't return")
	// 		return
	// 	case <-done:
	// 	}

	// })

	// t.Run("Master routine kills routines if user sends terminate:cancel property", func(t *testing.T) {
	// 	// TODO
	// })

}
