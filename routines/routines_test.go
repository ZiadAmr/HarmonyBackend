package routines

import (
	"harmony/backend/model"
	"testing"
)

// fake `Routines` implementation that tracks the number of times each routine was called
type FakeRoutinesCallCounter struct {
	comeOnlineCount int // defaults to 0
	totalCount      int
}

func (r *FakeRoutinesCallCounter) ComeOnline( /*need to copy the exact arguments here because fml. You can't use `...interface{}`*/ ) {
	r.comeOnlineCount++
	r.totalCount++
}

func TestMasterRoutine(t *testing.T) {

	t.Run("Master routine calls no routines when schema does not match", func(t *testing.T) {

		invalidMessages := []string{
			`{"initiate": "thisIsARoutineThatDoesNotExist"}`,
			`{}`,
			`this is not valid json`,
		}

		mockClient := &model.Client{
			PublicKey: "",
		}
		for _, tt := range invalidMessages {
			t.Run("test case: "+tt, func(t *testing.T) {
				mockTransaction := model.MakeTransaction()
				fakeRoutines := FakeRoutinesCallCounter{}

				// send non-matching json
				mockTransaction.FromCl <- tt

				// run function
				MasterRoutine(&fakeRoutines, mockClient, mockTransaction.FromCl, mockTransaction.ToCl)

				if fakeRoutines.totalCount != 0 {
					t.Errorf("Total routine call count: expected %v got %v", 0, fakeRoutines.totalCount)
				}
			})
		}

	})

	t.Run("Master routine calls only ComeOnline when schema matches", func(t *testing.T) {

		mockClient := &model.Client{
			PublicKey: "",
		}
		mockTransaction := model.MakeTransaction()
		fakeRoutines := &FakeRoutinesCallCounter{}

		// send matching schema
		mockTransaction.FromCl <- `{
    "initiate": "comeOnline"
}`
		// run function
		MasterRoutine(fakeRoutines, mockClient, mockTransaction.FromCl, mockTransaction.ToCl)

		// check only the correct routines was called
		if fakeRoutines.comeOnlineCount != 1 {
			t.Errorf("ComeOnline call count: expected %v got %v", 1, fakeRoutines.comeOnlineCount)
		}
		if fakeRoutines.totalCount != 1 {
			t.Errorf("Total routine call count: expected %v got %v", 1, fakeRoutines.totalCount)
		}

	})

}
