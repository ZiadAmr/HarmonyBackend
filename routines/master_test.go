package routines

import (
	"harmony/backend/model"
	"testing"
)

// stores all messages passed to it
type LoggerRoutine struct {
	msgs []string
}

func (r *LoggerRoutine) Next(args model.RoutineInput) []model.RoutineOutput {
	r.msgs = append(r.msgs, args.Msg)
	return []model.RoutineOutput{model.MakeRoutineOutput(false)}
}

func TestMasterRoutine(t *testing.T) {

	t.Run("Master routine calls no routines and returns error when schema does not match", func(t *testing.T) {

		invalidMessages := []string{
			`{"initiate": "thisIsARoutineThatDoesNotExist"}`,
			`{}`,
			`this is not valid json`,
		}

		for _, tt := range invalidMessages {
			t.Run(tt, func(t *testing.T) {

				callCount := 0

				incrementCallCount := func(c *model.Client, h *model.Hub) model.Routine {
					callCount += 1
					return &EmptyRoutine{}
				}

				// mock the routine constructors
				routineImpls := RoutineConstructors{
					NewComeOnline:                incrementCallCount,
					NewEstablishConnectionToPeer: incrementCallCount,
					NewFriendRequest:             incrementCallCount,
					NewFriendRejection:           incrementCallCount,
				}

				mockClient := &model.Client{}
				mockHub := model.NewHub()

				master := newMasterRoutineDependencyInj(routineImpls, mockClient, mockHub)

				testRunner(t, master, []Step{
					{
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Msg:     tt,
						},
						outputs: []ExpectedOutput{
							{
								ro: model.RoutineOutput{
									Msgs: []string{errorSchemaString()},
									Done: true,
								},
							},
						},
					},
				})

				if callCount != 0 {
					t.Errorf("Total routine call count: expected %v got %v", 0, callCount)
				}

			})
		}

	})

	t.Run("Master routine calls correct routine", func(t *testing.T) {

		tests := []struct {
			initiateKeyword        string
			routineConstructorName string
		}{
			{"comeOnline", "NewComeOnline"},
			{"sendConnectionRequest", "NewEstablishConnectionToPeer"},
			{"sendFriendRequest", "NewFriendRequest"},
			{"sendFriendRejection", "NewFriendRejection"},
		}

		for _, tt := range tests {
			t.Run(tt.initiateKeyword, func(t *testing.T) {

				calls := make([]string, 0)

				// mock the routine constructors to track new routines being created
				routineImpls := RoutineConstructors{
					NewComeOnline: func(c *model.Client, h *model.Hub) model.Routine {
						calls = append(calls, "NewComeOnline")
						return &EmptyRoutine{}
					},
					NewEstablishConnectionToPeer: func(c *model.Client, h *model.Hub) model.Routine {
						calls = append(calls, "NewEstablishConnectionToPeer")
						return &EmptyRoutine{}
					},
					NewFriendRequest: func(c *model.Client, h *model.Hub) model.Routine {
						calls = append(calls, "NewFriendRequest")
						return &EmptyRoutine{}
					},
					NewFriendRejection: func(c *model.Client, h *model.Hub) model.Routine {
						calls = append(calls, "NewFriendRejection")
						return &EmptyRoutine{}
					},
				}

				mockClient := &model.Client{}
				mockHub := model.NewHub()

				master := newMasterRoutineDependencyInj(routineImpls, mockClient, mockHub)
				master.Next(model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Pk:      nil,
					Msg: `{
						"initiate": "` + tt.initiateKeyword + `"
					}`,
				})

				// check only the correct routines was called
				thisRoutineCount := countOccurrences(calls, tt.routineConstructorName)
				totalCount := len(calls)

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

		test := []string{
			`{"initiate":"comeOnline"}`,
			"message 2",
			"message 3",
		}

		// mock comeOnline with a function that just logs all the msgs passed to it
		mockConstructorImpls := routineContructorImplementations
		loggerRoutine := &LoggerRoutine{}
		mockConstructorImpls.NewComeOnline = func(c *model.Client, h *model.Hub) model.Routine {
			return loggerRoutine
		}

		mockClient := &model.Client{}
		mockHub := model.NewHub()

		master := newMasterRoutineDependencyInj(mockConstructorImpls, mockClient, mockHub)

		for i, input := range test {
			master.Next(model.RoutineInput{
				MsgType: model.RoutineMsgType_UsrMsg,
				Pk:      nil,
				Msg:     input,
			})
			if len(loggerRoutine.msgs) != i+1 {
				t.Errorf("Input %s was not passed to routine", input)
				break
			}
			got := loggerRoutine.msgs[i]
			expected := input
			if got != expected {
				t.Errorf("Incorrect msg passed to function. Expected %s got %s", expected, got)
			}
		}

	})

}
