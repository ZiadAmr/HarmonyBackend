// helper functions common to tests in this package.
package routines

import (
	"harmony/backend/model"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

var publicKey0 = (model.PublicKey)("MCowBQYDK2VwAyEAUFRxKDllkUY843/zVOPE67zGqkGoMZd7dGKl2+9+pYQ=")

// var privateKey0 = "MC4CAQAwBQYDK2VwBCIEILLK2qyMQi162qzsJ2pV5bS5tX/6XEgWtw62eUKOKLAF"
//                    MC4CAQAwBQYDK2VwBCIEILLK2qyMQi162qzsJ2pV5cS5tX/6XEgWtw62eUKOKLAF

var publicKey1 = (model.PublicKey)("MCowBQYDK2VwAyEA1x5dCGTiFyoAGPP8XTzv58tZQHx5RB5E+5xFX5xwMFQ=")

// var privateKey1 = "MC4CAQAwBQYDK2VwBCIEIP192NwPoJrEi4IxNZRpYd5E9yoDQypY+3VNSuxSvFtn"

type ExpectedOutput struct {
	// json schemas instead of actual messages in the ro.
	ro             model.RoutineOutput
	verifyTimeouts bool
}

type Step struct {
	description string
	input       model.RoutineInput
	outputs     []ExpectedOutput
}

func pkToStr(pk *model.PublicKey) string {
	if pk == nil {
		return "nil"
	} else {
		return (string)(*pk)
	}
}

type testRunnerConfig struct {
	errorsOnLastStepOnly bool
}

func testRunner(t *testing.T, r model.Routine, steps []Step, configs ...testRunnerConfig) {

	var config testRunnerConfig

	if len(configs) >= 1 {
		config = configs[0]
	} else {
		config = testRunnerConfig{
			errorsOnLastStepOnly: false,
		}
	}

	var stepNum int
	var step Step

	// wrappers so error messages can be silenced more easily
	tErrorf := func(format string, args ...any) {
		if !(config.errorsOnLastStepOnly && stepNum != len(steps)-1) {
			t.Errorf(format, args...)
		}
	}
	tLogf := func(format string, args ...any) {
		if !(config.errorsOnLastStepOnly && stepNum != len(steps)-1) {
			t.Logf(format, args...)
		}
	}

	// clients whose transaction socket has been terminated
	terminatedClients := make(map[model.PublicKey]struct{})
	nilClientTerminted := false

	activeClients := make(map[model.PublicKey]struct{})
	nilClientActive := false

	// the first message sent to a routine will be from the initiating client, which is active
	if len(steps) != 0 {
		initiatingPk := steps[0].input.Pk
		if initiatingPk == nil {
			nilClientActive = true
		} else {
			activeClients[*initiatingPk] = struct{}{}
		}
	}

	for stepNum, step = range steps {

		expectedOutputsRemaining := make([]ExpectedOutput, len(step.outputs))
		copy(expectedOutputsRemaining, step.outputs)

		tLogf("Step %d: %s", stepNum, step.description)

		ros := r.Next(step.input)

		// replace all nil public keys with the key of the step initiator
		for i, ro := range ros {
			if ro.Pk == nil {
				ros[i].Pk = step.input.Pk
			}
		}

		// if a client closes connection
		if step.input.MsgType == model.RoutineMsgType_ClientClose {
			if step.input.Pk == nil {
				nilClientTerminted = true
				nilClientActive = false
			} else {
				terminatedClients[*step.input.Pk] = struct{}{}
				delete(activeClients, *step.input.Pk)
			}
		}

		// expect at most 1 RoutineOutput per client
		pksSeen := make(map[model.PublicKey]struct{}) // a set.
		nilPkSeen := false

		for _, ro := range ros {

			// expect at most 1 RoutineOutput per client
			if ro.Pk == nil {
				if nilPkSeen {
					tErrorf("Saw nil pk more than once in output of step %d", stepNum)
				} else {
					nilPkSeen = true
				}
			} else {
				_, pkSeen := pksSeen[*ro.Pk]
				if pkSeen {
					tErrorf("Saw pk %v more than once in output of step %d", *ro.Pk, stepNum)
				} else {
					pksSeen[*ro.Pk] = struct{}{}
				}
			}

			// RoutineOutput should not be sent to a terminated client
			if ro.Pk == nil {
				if nilClientTerminted {
					tErrorf("Sent RoutineOutput to terminated nil client in step %d", stepNum)
				}
			} else {
				_, terminated := terminatedClients[*ro.Pk]
				if terminated {
					tErrorf("Sent RoutineOutput to terminated client %s in step %d", pkToStr(ro.Pk), stepNum)
				}
			}

			// update active and terminated clients
			if ro.Pk == nil {
				if ro.Done {
					nilClientActive = false
					nilClientTerminted = true
				} else {
					nilClientActive = true
				}
			} else {
				if ro.Done {
					delete(activeClients, *ro.Pk)
					terminatedClients[*ro.Pk] = struct{}{}
				} else {
					activeClients[*ro.Pk] = struct{}{}
				}
			}

			// find the expected output
			var expectedOutput *ExpectedOutput
			for i, eo := range expectedOutputsRemaining {
				if (eo.ro.Pk == nil && ro.Pk == nil) ||
					(eo.ro.Pk != nil && ro.Pk != nil && *eo.ro.Pk == *ro.Pk) {
					expectedOutput = &eo
					// remove from list
					expectedOutputsRemaining = append(expectedOutputsRemaining[:i], expectedOutputsRemaining[i+1:]...)
					break
				}
			}

			if expectedOutput == nil {
				tErrorf("Unexpected output to pk %s in step %d, got %v", pkToStr(ro.Pk), stepNum, ro)
				continue
			}

			// compare timeouts
			if expectedOutput.verifyTimeouts {
				if !expectedOutput.ro.TimeoutEnabled {
					if ro.TimeoutEnabled {
						tErrorf("RoutineOutput to pk %s in step %d should not have timeout enabled. Got %v", pkToStr(ro.Pk), stepNum, ro)
					}
				} else {
					if !ro.TimeoutEnabled || ro.TimeoutDuration != expectedOutput.ro.TimeoutDuration {
						tErrorf("RoutineOutput to pk %s in step %d should have had a timeout of duration %v. Got %v", pkToStr(ro.Pk), stepNum, expectedOutput.ro.TimeoutDuration, ro)
					}
				}
			}

			// compare messages against schema
			numMsgsExpected := len(expectedOutput.ro.Msgs)
			numMsgsGot := len(ro.Msgs)
			if numMsgsExpected != numMsgsGot {
				tErrorf("Expected %d message(s) for pk %s in step %d. Got %d message(s)", numMsgsExpected, pkToStr(ro.Pk), stepNum, numMsgsGot)
			}
			for i := 0; i < min(numMsgsExpected, numMsgsGot); i++ {
				msgGot := ro.Msgs[i]
				msgExpectedSchema := expectedOutput.ro.Msgs[i]
				schemaLoader := gojsonschema.NewStringLoader(msgExpectedSchema)
				outputLoader := gojsonschema.NewStringLoader(msgGot)

				result, err := gojsonschema.Validate(schemaLoader, outputLoader)

				if err != nil {
					tErrorf("%s. Expected message %d sent to client with pk %s in step %d to match schema: %s\nGot: %s", err.Error(), i, pkToStr(ro.Pk), stepNum, msgExpectedSchema, msgGot)
				} else if !result.Valid() {
					t.Errorf("%s. Expected message %d sent to client with pk %s in step %d to match schema: %s\nGot: %s", formatJSONError(result), i, pkToStr(ro.Pk), stepNum, msgExpectedSchema, msgGot)
				}
			}

			// compare Done
			if expectedOutput.ro.Done != ro.Done {
				tErrorf("Expected client RoutineOutput to client %s in step %d to have done=%v. Got %v", pkToStr(ro.Pk), stepNum, expectedOutput.ro.Done, ro.Done)
			}
		}

		// check that all expected ros were sent
		// if there are some outputs left over here then they were not fulfilled
		for _, eo := range expectedOutputsRemaining {
			tErrorf("Expected a RoutineOutput for pk %s in step %d", pkToStr(eo.ro.Pk), stepNum)
		}

	}

	// check that all transaction sockets have been closed
	if nilClientActive {
		tErrorf("Transaction socket to nil client still open after test")
	}
	for pk := range activeClients {
		tErrorf("Transaction socket to client %s still open after test", pkToStr(&pk))
	}

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

// error messages to send to the client should look like this.

func errorSchemaString(msg ...string) string {
	var errorSchemaFragment string
	if len(msg) > 0 {
		errorSchemaFragment = `"const":"` + msg[0] + `"`
	} else {
		errorSchemaFragment = `"type":"string"`
	}
	return `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"terminate": {
				"const":"cancel"
			},
			"error": {
				` + errorSchemaFragment + `
			}
		},
		"required": ["terminate"],
		"additionalProperties": false
	}`
}

// minimum impl to satisfy the interface.
// doesn't do anything
type EmptyRoutine struct{}

func (r *EmptyRoutine) Next(args model.RoutineInput) []model.RoutineOutput {
	return []model.RoutineOutput{model.MakeRoutineOutput(false)}
}
