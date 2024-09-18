package routines

import (
	"harmony/backend/model"
	"testing"
	"time"
)

// returns a channel that will send a message after 10ms.
// can use this in a select statement to check for timeouts.
func shortTimePassed() <-chan time.Time {
	return time.After(10 * time.Millisecond)
}

// ==================================================================

// fake `Routines` implementation that tracks the names of the subroutine methods that were called
type FakeRoutinesCallTracker struct {
	calls []string
}

func (r *FakeRoutinesCallTracker) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string, kill chan struct{}) {
	r.calls = append(r.calls, "ComeOnline")
}
func (r *FakeRoutinesCallTracker) EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string, kill chan struct{}) {
	r.calls = append(r.calls, "EstablishConnectionToPeer")
}

//===================================================================

// another fake `routines` implementation used to mock the routines for various other behaviours
type FakeRoutines struct {
}

// raise an obvious error
func (r *FakeRoutines) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string, kill chan struct{}) {
	<-fromCl
	panic("testing...")
}

// timeout
func (r *FakeRoutines) EstablishConnectionToPeer(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string, kill chan struct{}) {
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
