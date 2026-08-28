package routines

import (
	"bytes"
	"harmony/backend/model"
	"io"
	"log/slog"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

// stores all messages passed to it
type LoggerRoutine struct {
	msgs []string
}

func (r *LoggerRoutine) Next(args model.RoutineInput) []model.RoutineOutput {
	r.msgs = append(r.msgs, args.Msg)
	return []model.RoutineOutput{model.MakeRoutineOutput(false)}
}

const masterRoutineName = "masterRoutine"

var masterRLogAttrs = toAnySlice("ip", ip0, "pk", string(publicKey1), "routine", masterRoutineName, "tsid", tsid1)

var expectedClientLogAttrs = ClientLogAttributes{
	ip: ip0,
	pk: string(publicKey1),
}

func TestMasterRoutine(t *testing.T) {

	t.Run("Master routine calls no routines and returns+logs error when schema does not match", func(t *testing.T) {

		invalidMessages := []string{
			`{"initiate": "thisIsARoutineThatDoesNotExist"}`,
			`{}`,
			`this is not valid json`,
		}

		for _, tt := range invalidMessages {
			t.Run(tt, func(t *testing.T) {

				callCount := 0

				incrementCallCount := func(c *model.Client, h *model.Hub, l *slog.Logger) model.Routine {
					callCount += 1
					return &TerminateImmediatelyRoutine{}
				}

				// mock the routine constructors
				routineImpls := RoutineConstructors{
					NewComeOnline:                incrementCallCount,
					NewEstablishConnectionToPeer: incrementCallCount,
					NewFriendRequest:             incrementCallCount,
					NewFriendRejection:           incrementCallCount,
				}

				mockClient := &model.Client{IpAddr: ip0}
				mockHub := model.NewHub(testAllowedHostnames)
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(masterRLogAttrs...)

				master := newMasterRoutineDependencyInj(routineImpls, mockClient, mockHub, mockLogger)

				testRunner(t, master, &logOutput, []Step{
					{
						description: "Invalid initiate message",
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
						logs: []ExpectedLog{
							{
								level:  "INFO",
								kind:   ROUTINE_FAIL,
								client: &expectedClientLogAttrs,
								transaction: &TransactionLogAttributes{
									routine: "masterRoutine",
									tsid:    tsid1,
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

	t.Run("Master routine calls+logs correct routine", func(t *testing.T) {

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

				// logger passed to sub routine - should have some attrs changed
				var subLogger *slog.Logger

				// mock the routine constructors to track new routines being created
				routineImpls := RoutineConstructors{
					NewComeOnline: func(c *model.Client, h *model.Hub, l *slog.Logger) model.Routine {
						calls = append(calls, "NewComeOnline")
						subLogger = l
						return &TerminateImmediatelyRoutine{}
					},
					NewEstablishConnectionToPeer: func(c *model.Client, h *model.Hub, l *slog.Logger) model.Routine {
						calls = append(calls, "NewEstablishConnectionToPeer")
						subLogger = l
						return &TerminateImmediatelyRoutine{}
					},
					NewFriendRequest: func(c *model.Client, h *model.Hub, l *slog.Logger) model.Routine {
						calls = append(calls, "NewFriendRequest")
						subLogger = l
						return &TerminateImmediatelyRoutine{}
					},
					NewFriendRejection: func(c *model.Client, h *model.Hub, l *slog.Logger) model.Routine {
						calls = append(calls, "NewFriendRejection")
						subLogger = l
						return &TerminateImmediatelyRoutine{}
					},
				}

				mockClient := &model.Client{IpAddr: ip0}
				mockHub := model.NewHub(testAllowedHostnames)
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(masterRLogAttrs...)
				master := newMasterRoutineDependencyInj(routineImpls, mockClient, mockHub, mockLogger)

				testRunner(t, master, &logOutput, []Step{
					{
						description: "Initiate " + tt.initiateKeyword,
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      nil,
							Msg: `{
								"initiate": "` + tt.initiateKeyword + `"
							}`,
						},
						outputs: []ExpectedOutput{
							// terminates immediately (mocked routine)
							{
								ro: model.RoutineOutput{
									Done: true,
								},
							},
						},
						logs: []ExpectedLog{
							{
								level:  "INFO",
								kind:   ROUTINE_INIT,
								client: &expectedClientLogAttrs,
								transaction: &TransactionLogAttributes{
									routine: masterRoutineName,
									tsid:    tsid1,
								},
							},
						},
					},
				})

				// check only the correct sub routine was called
				thisRoutineCount := countOccurrences(calls, tt.routineConstructorName)
				totalCount := len(calls)
				if thisRoutineCount != 1 {
					t.Errorf("Call count: expected %v got %v", 1, thisRoutineCount)
				}
				if totalCount != 1 {
					t.Errorf("Total routine call count: expected %v got %v", 1, totalCount)
				}

				// check that the logger passed to the sub routine has the correct attrs
				if subLogger == nil {
					t.Errorf("Logger passed to sub is nil")
				} else {
					// log something and test the output message
					logOutput.Reset()
					subLogger.Info("hello", "kind", "TEST")
					got, err := logOutput.ReadString('\n')
					if err != nil {
						t.Errorf("Error reading test log message")
					}
					schemaStr := expectedLogToSchema(ExpectedLog{
						level:  "INFO",
						kind:   "TEST",
						client: &expectedClientLogAttrs,
						transaction: &TransactionLogAttributes{
							routine: tt.initiateKeyword,
							tsid:    tsid1,
						},
						msg: "hello",
					})
					schemaLoader := gojsonschema.NewStringLoader(schemaStr)
					schema, err := gojsonschema.NewSchema(schemaLoader)
					if err != nil {
						t.Errorf("Problem with schema %s: %s", schemaStr, err.Error())
					}
					strLoader := gojsonschema.NewStringLoader(got)
					result, err := schema.Validate(strLoader)
					if err != nil {
						t.Errorf("%s. Expected test log to match schema: %s\nGot: %s", err.Error(), schemaStr, got)
					} else if !result.Valid() {
						t.Errorf("%s. Expected test log to match schema: %s\nGot: %s", formatJSONError(result), schemaStr, got)
					}
					if logOutput.Len() > 0 {
						t.Errorf("Expected no more log messages")
					}
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
		mockConstructorImpls.NewComeOnline = func(c *model.Client, h *model.Hub, l *slog.Logger) model.Routine {
			return loggerRoutine
		}

		mockClient := &model.Client{}
		mockHub := model.NewHub(testAllowedHostnames)
		master := newMasterRoutineDependencyInj(mockConstructorImpls, mockClient, mockHub, slog.New(slog.NewTextHandler(io.Discard, nil)))

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
