package routines

import (
	"harmony/backend/model"
	"testing"
	"time"
)

// returns a channel that will send a message after 10ms.
// can use this in a select statement to check for timeouts.
func waitForHang() <-chan time.Time {
	return time.After(10 * time.Millisecond)
}

// fake `Routines` implementation that tracks the tracks the names of the subroutine methods that were called
type FakeRoutinesCallTracker struct {
	calls []string
}

func (r *FakeRoutinesCallTracker) ComeOnline( /*need to copy the exact arguments here because fml. You can't use `...interface{}`*/ ) {
	r.calls = append(r.calls, "ComeOnline")
}
func (r *FakeRoutinesCallTracker) EstablishConnectionToPeer() {
	r.calls = append(r.calls, "EstablishConnectionToPeer")
}

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

		mockClient := &model.Client{
			PublicKey: nil,
		}
		for _, tt := range invalidMessages {
			t.Run(tt, func(t *testing.T) {
				mockTransaction := model.MakeTransaction()
				fakeRoutines := FakeRoutinesCallTracker{}

				// send non-matching json
				mockTransaction.FromCl <- tt

				// run function
				MasterRoutine(&fakeRoutines, mockClient, mockTransaction.FromCl, mockTransaction.ToCl)

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
				mockTransaction := model.MakeTransaction()
				fakeRoutines := &FakeRoutinesCallTracker{}

				// send matching schema
				mockTransaction.FromCl <- `{
					"initiate": "` + tt.initiateKeyword + `"
				}`
				// run function
				MasterRoutine(fakeRoutines, mockClient, mockTransaction.FromCl, mockTransaction.ToCl)

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
	// 	// TODO
	// })

	// t.Run("Master routine kills routines if user sends terminate:cancel property", func(t *testing.T) {
	// 	// TODO
	// })

}
