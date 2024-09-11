package routines

import (
	"encoding/hex"
	"harmony/backend/model"
	"strconv"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

const comeOnlineVersionResponseSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"version": {"type": "string"}
	},
	"required": ["version"],
	"additionalProperties": false
}`

const comeOnlineWelcomeResponseSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "welcome": {"const": "welcome"},
    "terminate": {"const": "done"}
  },
  "required": ["welcome", "terminate"],
  "additionalProperties": false
}`

type Step struct {
	kind    string
	content string
}

// feed inputs and validate output over the two channels.
// stops when a message is sent to `done`. Requires a `done` message following the last step
func runSteps(steps []Step, fromCl chan string, toCl chan string, done chan struct{}, t *testing.T) {
	for _, step := range steps {
		switch step.kind {
		case "input":
			select {
			case <-shortTimePassed():
				t.Errorf("Timeout on input:\n%s", step.content)
				return
			case fromCl <- step.content:
			case <-done:
				return
			}
		case "outputSchema":
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

func TestComeOnline(t *testing.T) {

	t.Run("runs correctly on valid inputs", func(t *testing.T) {

		// define tests
		tests := []struct {
			key   model.PublicKey
			steps []Step
		}{
			{
				key: func() model.PublicKey {
					key, _ := hex.DecodeString("cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e")
					return (model.PublicKey)(key)
				}(),
				steps: []Step{
					{
						"input",
						`{"initiate": "comeOnline"}`,
					},
					{
						"outputSchema",
						comeOnlineVersionResponseSchema,
					},
					{
						"input",
						`{
							"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"
						}`,
					},
					{
						"outputSchema",
						comeOnlineWelcomeResponseSchema,
					},
				},
			},
		}

		for i, tt := range tests {

			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				mockClient := &model.Client{}
				r := RoutinesDefn{}
				FromCl := make(chan string)
				ToCl := make(chan string)
				ErrCl := make(chan string, 1)
				defer close(ErrCl)

				// use to check that routine has returned/finished.
				// struct{} means nothing - don't actually pass any value
				done := make(chan struct{})

				go func() {
					defer close(FromCl)
					defer close(ToCl)
					r.ComeOnline(mockClient, FromCl, ToCl, ErrCl)
					done <- struct{}{}
				}()

				// pass test inputs in and validate outputs
				runSteps(tt.steps, FromCl, ToCl, done, t)

				// expect no errors
				select {
				case err := <-ErrCl:
					t.Errorf(err)
					return
				default:
				}

				// check that the key has been updated as expected
				if *mockClient.GetPublicKey() != tt.key {
					t.Errorf("public key not correct: expected %s got %s", tt.key, *mockClient.GetPublicKey())
				}
			})
		}
	})

	t.Run("cancels transaction on bad public key message", func(t *testing.T) {

		tests := []struct {
			steps []Step
		}{
			{
				steps: []Step{
					{
						"input",
						`{"initiate": "comeOnline"}`,
					},
					{
						"outputSchema",
						comeOnlineVersionResponseSchema,
					},
					{
						"input",
						`bad input!!!!!`,
					},
					{
						"outputSchema",
						comeOnlineWelcomeResponseSchema,
					},
				},
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				mockClient := &model.Client{}
				r := RoutinesDefn{}
				FromCl := make(chan string)
				ToCl := make(chan string)
				ErrCl := make(chan string, 1)
				defer close(ErrCl)

				// use to check that routine has returned/finished.
				// struct{} means nothing - don't actually pass any value
				done := make(chan struct{})
				defer close(done)

				go func() {
					defer close(FromCl)
					defer close(ToCl)
					r.ComeOnline(mockClient, FromCl, ToCl, ErrCl)
					done <- struct{}{}
				}()

				// pass test inputs in and validate outputs
				runSteps(tt.steps, FromCl, ToCl, done, t)

				// check to see if there was an error
				select {
				case <-ErrCl:
				default:
					t.Errorf("Expected an error.")
				}
			})
		}
	})

	// t.Run("cancels transaction if public key is already signed in", func(t *testing.T) {

	// 	steps := []Step{
	// 		{
	// 			"input",
	// 			`{"initiate": "comeOnline"}`,
	// 		},
	// 		{
	// 			"outputSchema",
	// 			comeOnlineVersionResponseSchema,
	// 		},
	// 		{
	// 			"input",
	// 			`{
	// 				"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"
	// 			}`,
	// 		},
	// 		// no final step because it should fail
	// 	}

	// 	// mock hub with the client already signed in

	// 	// model.NewGenericHub[Model]()

	// })

}

func TestParseUserKeyMessage(t *testing.T) {
	t.Run("accepts correct keys and parses correctly", func(t *testing.T) {
		tests := []struct {
			keyStr string
			key    model.PublicKey
		}{
			{
				`{
					"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"
				}`,
				(model.PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e")),
			},
		}

		for _, tt := range tests {

			t.Run(tt.keyStr, func(t *testing.T) {

				result, err := parseUserKeyMessage(tt.keyStr)

				if err != nil {
					t.Errorf(err.Error())
					return
				}

				if *result != tt.key {
					t.Errorf("Expected %v got %v", tt.key, *result)
				}
			})

		}
	})

	t.Run("rejects public keys in incorrect format", func(t *testing.T) {

		tests := []struct {
			keyStr string
		}{
			{`{
				"publicKey": "illegal_characters______________________________________________________________________________________________________________"
			}`},
			{`{
				"publicKey": "0123456789abcdef"
			}`},
			{`{
				"publicKey": "0123456789abcde"
			}`},
			{`{}`},
			{`{
				"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e",
				"extraUnwantedProperty": "boo!"
			}`},
			{`{
				"publicKey": false,
			}`},
			{"cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"},
		}

		for _, tt := range tests {
			t.Run(tt.keyStr, func(t *testing.T) {
				result, err := parseUserKeyMessage(tt.keyStr)

				if err == nil {
					t.Errorf("Expected an error to be returned. Instead got value %v", *result)
					return
				}

			})
		}
	})
}
