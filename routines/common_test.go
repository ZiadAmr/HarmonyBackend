// helper functions common to tests in this package.
package routines

import (
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
	kind    StepKind
	content string
}

func testRunner(t *testing.T, r model.Routine, steps []Step) {
	// queue of messages to be sent to the client
	msgsToClient := make([]string, 0)
	done := false

	for _, step := range steps {

		switch step.kind {
		case step_input:

			if len(msgsToClient) > 0 {
				// there are still messages in the queue to send to the client
				t.Errorf("Unexpected message(s) to client: %v", msgsToClient)
			}
			if done {
				t.Errorf("Unexpected message from client (routine has terminated): %s", step.content)
			} else {
				stepOutput := r.Next(step.content)
				msgsToClient = stepOutput.Msgs
				done = stepOutput.Done
			}
		case step_outputSchema:
			if len(msgsToClient) == 0 {
				t.Errorf("Expected a message to client matching schema %s", step.content)
			} else {
				// pop msg
				msg := msgsToClient[0]
				msgsToClient = msgsToClient[1:]

				schemaLoader := gojsonschema.NewStringLoader(step.content)
				outputLoader := gojsonschema.NewStringLoader(msg)

				result, err := gojsonschema.Validate(schemaLoader, outputLoader)

				if err != nil {
					t.Errorf("%s. Expected to match schema: %s\nGot: %s", err.Error(), step.content, msg)
				}

				if !result.Valid() {
					t.Errorf("%s. Expected to match schema: %s\nGot: %s", formatJSONError(result), step.content, msg)
				}
			}
		}
	}

	if len(msgsToClient) > 0 {
		t.Errorf("Unexpected message(s) to send to client at end of routine: %v", msgsToClient)
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

func (r *EmptyRoutine) Next(msg string) model.RoutineOutput {
	return model.MakeRoutineOutput(false)
}
