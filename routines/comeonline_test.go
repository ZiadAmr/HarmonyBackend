package routines

import (
	"encoding/hex"
	"harmony/backend/model"
	"strconv"
	"testing"
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
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Msg:     `{"initiate": "comeOnline"}`,
						},
						outputs: []ExpectedOutput{
							{
								ro: model.RoutineOutput{
									Msgs: []string{comeOnlineVersionResponseSchema},
								},
							},
						},
					},
					{
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Msg: `{
								"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"
							}`,
						},
						outputs: []ExpectedOutput{
							{
								ro: model.RoutineOutput{
									Msgs: []string{comeOnlineWelcomeResponseSchema},
									Done: true,
								},
							},
						},
					},
				},
			},
		}

		for i, tt := range tests {

			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				mockClient := &model.Client{}
				mockHub := model.NewHub()

				co := newComeOnline(mockClient, mockHub)

				testRunner(t, co, tt.steps)

				// check that the key has been updated as expected
				if mockClient.GetPublicKey() == nil {
					t.Errorf("Expected public key of client not to be nil")
				} else if *mockClient.GetPublicKey() != tt.key {
					t.Errorf("public key not correct: expected %s got %s", tt.key, *mockClient.GetPublicKey())
				}

				// check that the client has been added to the hub
				hubClient, exists := mockHub.GetClient(tt.key)
				if !exists {
					t.Errorf("Expected client to be added to the hub")
				} else if hubClient != mockClient {
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
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Msg:     `{"initiate": "comeOnline"}`,
						},
						outputs: []ExpectedOutput{
							{
								ro: model.RoutineOutput{
									Msgs: []string{comeOnlineVersionResponseSchema},
								},
							},
						},
					},
					{
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Msg:     `bad input!!!!!`,
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
				},
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				mockClient := &model.Client{}
				mockHub := model.NewHub()

				co := newComeOnline(mockClient, mockHub)

				testRunner(t, co, tt.steps)
			})
		}
	})

	t.Run("cancels transaction if public key is already signed in", func(t *testing.T) {

		steps := []Step{
			{
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Msg:     `{"initiate": "comeOnline"}`,
				},
				outputs: []ExpectedOutput{
					{
						ro: model.RoutineOutput{
							Msgs: []string{comeOnlineVersionResponseSchema},
						},
					},
				},
			},
			{
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Msg: `{
						"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"
					}`,
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
		}

		// mock hub with the client already signed in

		hub := model.NewHub()

		client0 := &model.Client{}
		key := (model.PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))
		client0.SetPublicKey(&key)
		hub.AddClient(key, client0)

		// client that tries to use a public key that is already signed in
		client1 := &model.Client{}

		co := newComeOnline(client1, hub)

		testRunner(t, co, steps)

		// client id was not updated
		if client1.GetPublicKey() != nil {
			t.Errorf("Public key expected nil got %v", client1.GetPublicKey())
		}

		// hub still returns original client
		expected := client0
		got, _ := hub.GetClient(key)
		if expected != got {
			t.Errorf("Expected client not to be updated. Expected %v got %v", expected, got)
		}

	})

	t.Run("Rejects immediately if public key is already set", func(t *testing.T) {

		pk := (model.PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))

		steps := []Step{
			{
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Msg:     `{"initiate": "comeOnline"}`,
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
		}

		mockClient := &model.Client{}
		mockClient.SetPublicKey(&pk)

		mockHub := model.NewHub()

		co := newComeOnline(mockClient, mockHub)

		testRunner(t, co, steps)

	})

	t.Run(`Return immediately if receiving {"terminate","cancel"} from client`, func(t *testing.T) {

		tests := [][]Step{
			{
				{
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Msg:     `{"initiate": "comeOnline", "terminate":"cancel"}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Done: true,
							},
						},
					},
				},
			},
			{
				{
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Msg:     `{"initiate": "comeOnline"}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Msgs: []string{comeOnlineVersionResponseSchema},
							},
						},
					},
				},
				{
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Msg:     `{"terminate": "cancel"}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Done: true,
							},
						},
					},
				},
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				mockClient := &model.Client{}
				mockHub := model.NewHub()
				co := newComeOnline(mockClient, mockHub)
				testRunner(t, co, tt)
			})
		}
	})

	t.Run("does not send any more messages after a client close", func(t *testing.T) {
		tests := [][]Step{
			{
				{
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Msg:     `{"initiate": "comeOnline"}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Msgs: []string{comeOnlineVersionResponseSchema},
							},
						},
					},
				},
				{
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_ClientClose,
					},
					// expect no output
				},
			},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				mockClient := &model.Client{}
				mockHub := model.NewHub()
				co := newComeOnline(mockClient, mockHub)
				testRunner(t, co, tt)
			})
		}
	})

	t.Run("Closes after a timeout", func(t *testing.T) {
		test := []Step{
			{
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Msg:     `{"initiate": "comeOnline"}`,
				},
				outputs: []ExpectedOutput{
					{
						ro: model.RoutineOutput{
							Msgs: []string{comeOnlineVersionResponseSchema},
						},
					},
				},
			},
			{
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_Timeout,
				},
				outputs: []ExpectedOutput{
					{
						ro: model.RoutineOutput{
							Done: true,
							Msgs: []string{errorSchemaString("timeout")},
						},
					},
				},
			},
		}

		mockClient := &model.Client{}
		mockHub := model.NewHub()
		co := newComeOnline(mockClient, mockHub)
		testRunner(t, co, test)

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
