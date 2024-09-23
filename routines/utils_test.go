// helper functions common to tests in this package.
package routines

import (
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
func RunSteps(steps []Step, fromCl chan string, toCl chan string, done chan struct{}, t *testing.T) {
	for _, step := range steps {
		switch step.kind {
		case step_input:
			select {
			case <-shortTimePassed():
				t.Errorf("Timeout on input:\n%s", step.content)
				return
			case fromCl <- step.content:
			case <-done:
				return
			}
		case step_outputSchema:
			var output string
			select {
			case <-shortTimePassed():
				t.Errorf("Timeout waiting for output:\n%s", step.content)
				return
			case output = <-toCl:
			case <-done:
				return
			}

			// verify output against schema
			schemaLoader := gojsonschema.NewStringLoader(step.content)
			outputLoader := gojsonschema.NewStringLoader(output)

			result, err := gojsonschema.Validate(schemaLoader, outputLoader)

			if err != nil {
				t.Errorf(err.Error())
				return
			}

			if !result.Valid() {
				t.Errorf("%s. Got:\n%s", formatJSONError(result), output)
			}
		}
	}
	select {
	case <-shortTimePassed():
		t.Errorf("Timeout waiting for routine to end")
		return
	case <-done:
	}
}
