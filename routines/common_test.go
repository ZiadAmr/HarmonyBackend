// helper functions common to tests in this package.
package routines

import (
	"fmt"
	"harmony/backend/model"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

type StepKind int

const /* enum */ (
	step_input StepKind = iota
	step_outputSchema
)

type Step struct {
	nextMsgType model.RoutineMsgType
	pk          *model.PublicKey
	kind        StepKind
	content     string
}

func pkToStr(pk *model.PublicKey) string {
	if pk == nil {
		return "nil"
	} else {
		return fmt.Sprintf("%v", *pk)
	}
}

func testRunner(t *testing.T, r model.Routine, steps []Step) {
	pendingRoutineOutputs := make([]model.RoutineOutput, 0)

	// track which clients can still send messages
	doneMap := make(map[model.PublicKey]bool)
	// pk might not be set
	nilPkDone := false

	for _, step := range steps {
		switch step.kind {
		case step_input:
			if len(pendingRoutineOutputs) > 0 {
				t.Errorf("Unexpected messages sent to client with pk %s:, %s", pkToStr(step.pk), step.content)
			}

			pendingRoutineOutputs = r.Next(step.nextMsgType, step.pk, step.content)

			// expect at most 1 RoutineOutput per client
			pksSeen := make(map[model.PublicKey]struct{}) // a set.
			nilPkSeen := false

			for _, ro := range pendingRoutineOutputs {

				// public key of the routine output
				// if nil, this is a reply the the client that sent the input
				if ro.Pk == nil {
					ro.Pk = step.pk
				}

				// update `done` for each RoutineOutput
				if ro.Pk == nil {
					if nilPkSeen {
						t.Errorf("Saw nil pk more than once. Got msgs %v", ro.Msgs)
					}
					if nilPkDone {
						t.Errorf("nil pk already done with this transaction. Got msgs %v", ro.Msgs)
					}
					nilPkDone = ro.Done
					nilPkSeen = true
				} else {
					_, pkSeen := pksSeen[*ro.Pk]
					if pkSeen {
						t.Errorf("Saw pk %v more than once. Got msgs %v", *ro.Pk, ro.Msgs)
					}
					done, inDoneMap := doneMap[*ro.Pk]
					if inDoneMap && done {
						t.Errorf("Pk %v already done with this transaction. Got msgs %v", *ro.Pk, ro.Msgs)
					}
					doneMap[*ro.Pk] = ro.Done
					pksSeen[*ro.Pk] = struct{}{} // add to set
				}

			}

			// remove any RoutineOutputs that have 0 messages
			newPendingRoutineOutputs := []model.RoutineOutput{}
			for _, ro := range pendingRoutineOutputs {
				if len(ro.Msgs) != 0 {
					newPendingRoutineOutputs = append(newPendingRoutineOutputs, ro)
				}
			}
			pendingRoutineOutputs = newPendingRoutineOutputs

		case step_outputSchema:
			// client routine output
			var clientRo *model.RoutineOutput
			var clientRoIdx = 0
			for i, ro := range pendingRoutineOutputs {
				clientRoIdx = i
				if ro.Pk == nil && step.pk == nil {
					clientRo = &ro
					break
				} else if ro.Pk != nil && step.pk != nil && *step.pk == *ro.Pk {
					clientRo = &ro
					break
				}
			}
			if clientRo == nil {
				t.Errorf("Expected a message sent to client %s matching schema %s", pkToStr(step.pk), step.content)
				continue
			}

			// pop the first message
			msg := clientRo.Msgs[0]
			clientRo.Msgs = clientRo.Msgs[1:]

			// if no more messages, remove the ro
			if len(clientRo.Msgs) == 0 {
				pendingRoutineOutputs = append(pendingRoutineOutputs[:clientRoIdx], pendingRoutineOutputs[clientRoIdx+1:]...)
			}

			// verify message against schema
			schemaLoader := gojsonschema.NewStringLoader(step.content)
			outputLoader := gojsonschema.NewStringLoader(msg)

			result, err := gojsonschema.Validate(schemaLoader, outputLoader)

			if err != nil {
				t.Errorf("%s. Expected message sent to client with pk %s to match schema: %s\nGot: %s", pkToStr(step.pk), err.Error(), step.content, msg)
			}

			if !result.Valid() {
				t.Errorf("%s. Expected message sent to client with pk %s to match schema: %s\nGot: %s", pkToStr(step.pk), formatJSONError(result), step.content, msg)
			}

		}

	}

	if len(pendingRoutineOutputs) > 0 {
		t.Errorf("Unexpected message(s) to send to client(s) at end of routine: %v", pendingRoutineOutputs)
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

const errorSchemaString = `
	{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"terminate": {
				"const":"cancel"
			},
            "error": {
             	"type": "string" 
            }
		},
		"required": ["terminate"],
		"additionalProperties": false
	}
`

// minimum impl to satisfy the interface.
// doesn't do anything
type EmptyRoutine struct{}

func (r *EmptyRoutine) Next(msgType model.RoutineMsgType, pk *model.PublicKey, msg string) []model.RoutineOutput {
	return []model.RoutineOutput{model.MakeRoutineOutput(false)}
}
