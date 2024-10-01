// helper functions common to tests in this package.
package routines

import (
	"harmony/backend/model"
	"testing"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

// returns a channel that will send a message after 10ms.
// can use this in a select statement to check for timeouts.
func shortTimePassed() <-chan time.Time {
	return time.After(10 * time.Millisecond)
}

type StepKind int

const /* enum */ (
	step_input StepKind = iota
	step_outputSchema
)

type Step struct {
	kind    StepKind
	content string
}

// feed inputs and validate output over the two channels.
// stops when a message is sent to `done`. Requires a `done` message following the last step
// if the routine sends an error `{"terminate":"cancel"}` then return the error
func RunSteps(steps []Step, fromCl chan string, toCl chan string, done chan struct{}, t *testing.T) *string {

	var output string

	for _, step := range steps {
		switch step.kind {
		case step_input:
			select {
			case <-shortTimePassed():
				t.Errorf("Timeout on input:\n%s", step.content)
				return nil
			case fromCl <- step.content: // send message
			case output = <-toCl: // receive message unexpectedly
				if isErrorMessage(output) {
					return &output
				} else {
					t.Errorf("Unexpected output: %s", output)
					return nil
				}
			case <-done:
				return nil
			}
		case step_outputSchema:
			select {
			case <-shortTimePassed():
				t.Errorf("Timeout waiting for output:\n%s", step.content)
				return nil
			case <-done:
				return nil
			case output = <-toCl:
				// continues outside select statement...
			}

			if isErrorMessage(output) {
				return &output
			}

			// verify output against schema
			schemaLoader := gojsonschema.NewStringLoader(step.content)
			outputLoader := gojsonschema.NewStringLoader(output)

			result, err := gojsonschema.Validate(schemaLoader, outputLoader)

			if err != nil {
				t.Errorf(err.Error())
				return nil
			}

			if !result.Valid() {
				t.Errorf("%s. Got:\n%s", formatJSONError(result), output)
			}
		}
	}
	select {
	case <-shortTimePassed():
		t.Errorf("Timeout waiting for routine to end")
		return nil
	case output = <-toCl: // receive message unexpectedly
		if isErrorMessage(output) {
			return &output
		} else {
			t.Errorf("Unexpected output: %s", output)
			return nil
		}
	case <-done:
		return nil
	}
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

var errorSchema = func() *gojsonschema.Schema {
	schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func isErrorMessage(msg string) bool {
	msgLoader := gojsonschema.NewStringLoader(msg)
	result, err := errorSchema.Validate(msgLoader)
	return err == nil && result.Valid()
}

// minimum impl to satisfy the interface.
// doesn't do anything
type EmptyRoutine struct{}

func (r *EmptyRoutine) Next(msg string) model.RoutineOutput {
	return model.MakeRoutineOutput(false)
}
