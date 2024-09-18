package routines

import (
	"encoding/hex"
	"harmony/backend/model"
	"strconv"
	"testing"
	"time"

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

// timeout inside the comeonline function
// set to high value so it can be ignored except for when being explicitly tested.
const internalTimeout = 1 * time.Minute

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

// make channels, then run steps on ComeOnline function
func comeOnlineTestWrapper(timeout time.Duration, t *testing.T, client *model.Client, hub *model.Hub, steps []Step) chan string {
	FromCl := make(chan string)
	ToCl := make(chan string)
	ErrCl := make(chan string, 1)
	kill := make(chan struct{})
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(FromCl)
		defer close(ToCl)
		defer close(kill)
		comeOnlineDependencyInj(timeout, client, hub, FromCl, ToCl, ErrCl, kill)
		done <- struct{}{}
	}()
	// pass test inputs in and validate outputs
	runSteps(steps, FromCl, ToCl, done, t)
	return ErrCl
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
				mockHub := model.NewHub()

				errCl := comeOnlineTestWrapper(internalTimeout, t, mockClient, mockHub, tt.steps)
				defer close(errCl)

				// expect no errors
				select {
				case err := <-errCl:
					t.Errorf(err)
					return
				default:
				}

				// check that the key has been updated as expected
				if *mockClient.GetPublicKey() != tt.key {
					t.Errorf("public key not correct: expected %s got %s", tt.key, *mockClient.GetPublicKey())
				}

				// check that the client has been added to the hub
				hubClient, exists := mockHub.GetClient(tt.key)
				if !exists {
					t.Errorf("Expected client to be added to the hub")
				}
				if hubClient != mockClient {
					t.Errorf("Expected client pointer to be added to the hub. Expected %v got %v", mockClient, hubClient)
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
				mockHub := model.NewHub()

				errCl := comeOnlineTestWrapper(internalTimeout, t, mockClient, mockHub, tt.steps)
				defer close(errCl)

				// check to see if there was an error
				select {
				case <-errCl:
				default:
					t.Errorf("Expected an error.")
				}
			})
		}
	})

	t.Run("cancels transaction if public key is already signed in", func(t *testing.T) {

		steps := []Step{
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
			// no final step because it should fail
		}

		// mock hub with the client already signed in

		hub := model.NewHub()

		client0 := &model.Client{}
		key := (*model.PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))
		client0.SetPublicKey(key)
		hub.AddClient(client0)

		// client that tries to use a public key that is already signed in
		client1 := &model.Client{}

		errCl := comeOnlineTestWrapper(internalTimeout, t, client1, hub, steps)

		select {
		case <-errCl:
		default:
			t.Errorf("Expected an error.")
		}

		// client id was not updated
		if client1.GetPublicKey() != nil {
			t.Errorf("Public key expected nil got %v", client1.GetPublicKey())
		}

		// hub still returns original client
		expected := client0
		got, _ := hub.GetClient(*key)
		if expected != got {
			t.Errorf("Expected client not to be updated. Expected %v got %v", expected, got)
		}

	})

	t.Run("Rejects immediately if public key is already set", func(t *testing.T) {

		pk := (*model.PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))

		steps := []Step{}

		mockClient := &model.Client{}
		mockClient.SetPublicKey(pk)

		mockHub := model.NewHub()

		errCl := comeOnlineTestWrapper(internalTimeout, t, mockClient, mockHub, steps)
		defer close(errCl)

		select {
		case <-errCl:
		default:
			t.Errorf("Expected an error")
		}
	})

	t.Run("Cancels transation if the client takes too long to respond", func(t *testing.T) {
		steps := []Step{
			{
				"input",
				`{"initiate": "comeOnline"}`,
			},
			{
				"outputSchema",
				comeOnlineVersionResponseSchema,
			},
			// routine should be waiting for user input

		}

		mockClient := &model.Client{}
		mockHub := model.NewHub()

		errCl := comeOnlineTestWrapper(1*time.Microsecond, t, mockClient, mockHub, steps)

		select {
		case <-errCl:
		default:
			t.Errorf("Expected an error")
		}

	})

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
