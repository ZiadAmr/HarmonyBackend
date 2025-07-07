package routines

import (
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

// `message` should be escaped for json.
var comeOnlineSignThisResponseSchema = func(message ...string) string {
	var frag string
	if len(message) > 0 {
		frag = `{"const": "` + message[0] + `"}`
	} else {
		frag = `{"type": "string"}`
	}
	return `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "signThis": ` + frag + `
  },
  "required": ["signThis"],
  "additionalProperties": false
}`
}

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
			key       model.PublicKey
			msgToSign string
			steps     []Step
		}{
			{
				key:       publicKey0,
				msgToSign: testMessage,
				steps: []Step{
					coStepInitiate,
					coStepValidPk(publicKey0, testMessage),
					coStepValidSignature(testPk0Signature),
				},
			},
		}

		for i, tt := range tests {

			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				mockClient := &model.Client{}
				mockHub := model.NewHub()
				mockRndMsgGen := fixedMessageGenerator{tt.msgToSign}

				co := newComeOnlineDependencyInj(mockClient, mockHub, mockRndMsgGen)

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
			description  string
			prefaceSteps []Step
			cases        []Step
		}{
			{
				description: "Bad public key",
				prefaceSteps: []Step{
					coStepInitiate,
				},
				cases: []Step{
					coStepBadPublicKey(`{"publicKey": "illegal_characters______________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________________"}`),
					coStepBadPublicKey(`{"publicKey": "NBSWY3DPEE======"}`),
					coStepBadPublicKey(`{}`),
					coStepBadPublicKey(`{"publicKey": "` + (string)(publicKey0) + `","extraUnwantedProperty": "boo!"}`),
					coStepBadPublicKey(`{"publicKey": false}`),
					coStepBadPublicKey((string)(publicKey0)),
					coStepBadPublicKey(`{"publicKey": "0123456789ABCDE="}`, "public key is not ed25519"),
					coStepBadPublicKey(`{"publicKey": "MCoxBQYDK2VwAyEA6pf9wPoa7Y6zeuwENUOifdDYN9kmYrd4jWIa3032spU="}`, "public key is not ed25519" /*invalid key - character modified in the header*/),
					coStepBadPublicKey(`{"publicKey": "MEkwEwYHKoZIzj0CAQYIKoZIzj0DAQEDMgAEoGveud25v3hQMWyISkUboxNF/0dXLnTn1G4kmdmb44NMstp5bvxdXDrRg4F0l+ZK"}`, "public key is not ed25519" /*invalid key - uses NIST192p curve instead of Ed25519*/),
				},
			},
		}

		for _, test := range tests {

			for _, testCase := range test.cases {

				t.Run(test.description+"-"+testCase.input.Msg, func(t *testing.T) {

					mockClient := &model.Client{}
					mockHub := model.NewHub()
					co := newComeOnline(mockClient, mockHub)

					testRunner(t, co, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})
				})

			}
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
						"publicKey": "` + (string)(publicKey0) + `"
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
		key := publicKey0
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

	t.Run("Cancels if another comeOnline is in progress", func(t *testing.T) {
		// first comeonline. Run these steps without checking the result
		co0Tests := [][]Step{
			{
				coStepInitiate,
			},
			{
				coStepInitiate,
				coStepValidPk(publicKey0, testMessage),
			},
		}

		// second comeonline should fail, as the first is not completed.
		co1Test := []Step{
			{
				description: "Comeonline fails to start because another is in progress",
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Msg:     `{"initiate": "comeOnline"}`,
				},
				outputs: []ExpectedOutput{
					{
						ro: model.RoutineOutput{
							Done: true,
							Msgs: []string{errorSchemaString("Another comeOnline routine is in progress")},
						},
					},
				},
			},
		}

		for i, test := range co0Tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				client := &model.Client{}
				hub := model.NewHub()
				mockRndMsgGen := fixedMessageGenerator{testMessage}
				co0 := newComeOnlineDependencyInj(client, hub, mockRndMsgGen)

				// manually run the first test - after this point it is not complete
				for _, step := range test {
					co0.Next(step.input)
				}

				// start another comeOnline
				co1 := newComeOnlineDependencyInj(client, hub, mockRndMsgGen)
				testRunner(t, co1, co1Test) // expect it to fail
			})
		}
	})

	t.Run("Allows subsequent comeOnline's after a failed one", func(t *testing.T) {
		co0Tests := [][]Step{
			{
				coStepInitiate,
				coStepValidPk(publicKey0, testMessage),
				coStepInvalidSignature("!!!"),
			},
			{
				coStepInitiate,
				coStepBadPublicKey("!"),
			},
		}

		co1Test := []Step{
			coStepInitiate,
			coStepValidPk(publicKey0, testMessage),
			coStepValidSignature(testPk0Signature),
		}

		for i, test := range co0Tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				client := &model.Client{}
				hub := model.NewHub()
				mockRndMsgGen := fixedMessageGenerator{testMessage}
				co0 := newComeOnlineDependencyInj(client, hub, mockRndMsgGen)

				// manually run the first test - it has completed at this point.
				for _, step := range test {
					co0.Next(step.input)
				}

				// start another comeOnline
				co1 := newComeOnlineDependencyInj(client, hub, mockRndMsgGen)
				testRunner(t, co1, co1Test) // expect it not to fail
			})
		}
	})

	t.Run("Rejects immediately if public key is already set", func(t *testing.T) {

		pk := publicKey0

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
				coStepInitiate,
				coStepClientCancel,
			},
			{
				coStepInitiate,
				coStepValidPk(publicKey0),
				coStepClientCancel,
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
				coStepInitiate,
				coStepClientClose,
			},
			{
				coStepInitiate,
				coStepValidPk(publicKey0),
				coStepClientClose,
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
		tests := [][]Step{
			{
				coStepInitiate,
				coStepTimeout,
			},
			{
				coStepInitiate,
				coStepValidPk(publicKey0),
				coStepTimeout,
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

	t.Run("Rejects incorrect/invalid signatures signatures", func(t *testing.T) {

		tests := []struct {
			description  string
			publicKey    model.PublicKey
			msgToSign    string
			prefaceSteps []Step
			cases        []Step
		}{
			{
				description: "User sends an invalid signature with a valid public key",
				publicKey:   publicKey0,
				msgToSign:   testMessage,
				prefaceSteps: []Step{
					coStepInitiate,
					coStepValidPk(publicKey0, testMessage),
				},
				cases: []Step{
					coStepInvalidSignature(`{"signature":"_"}`),
					coStepInvalidSignature(`{"signature":4}`),
					coStepInvalidSignature(`{"signature":null}`),
					coStepInvalidSignature(`{}`),
					coStepInvalidSignature(`{"signature":"` + testPk0Signature + `", "extraProperty":"hello"}`),
					coStepInvalidSignature(`}`),
					coStepInvalidSignature(testPk0Signature /*no json wrapper*/),
					coStepInvalidSignature(`{"signature":"0000"}`, "Invalid signature"),
					coStepInvalidSignature(`{"signature":"jIX/9ZHy6UuGZzywconx5rSV77yGugYg2M40ROilWS/zo3qnlau2Zn2p045ZYdKDH98LrMm8vJOmdmWBCkY0Bg=="}`, "Invalid signature" /*One char modified in signature*/),
					coStepInvalidSignature(`{"signature":"Bzj4qPcKt/bgAfH+JN3CWqyD0X0djWXLh19Bk23yJxrVunVfC/yU9MP6ue/as7edxcY08xdoWjFKu5HYMeiGBQ=="}`, "Invalid signature" /*Signed with a different private key*/),
				},
			},
		}

		for _, test := range tests {

			for _, testCase := range test.cases {

				t.Run(test.description+"-"+testCase.input.Msg, func(t *testing.T) {

					mockClient := &model.Client{}
					mockHub := model.NewHub()
					mockRndMsgGen := fixedMessageGenerator{test.msgToSign}
					co := newComeOnlineDependencyInj(mockClient, mockHub, mockRndMsgGen)

					testRunner(t, co, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})

					// check that the key has NOT been updated
					if mockClient.GetPublicKey() != nil {
						t.Errorf("Expected public key of client to be nil")
					}

					// check that the client has NOT been added to the hub
					_, exists := mockHub.GetClient(test.publicKey)
					if exists {
						t.Errorf("Expected client not to be added to the hub")
					}
				})

			}
		}

	})

	t.Run("Random string generator", func(t *testing.T) {
		t.Run("Return value is not hard-coded", func(t *testing.T) {
			gen := RandomMessageGeneratorImpl{}
			str0, _ := gen.GetMessage()
			str1, _ := gen.GetMessage()
			str2, _ := gen.GetMessage()

			if str0 == str1 || str1 == str2 || str2 == str0 {
				t.Errorf(`Return value appears to be hard-coded. From 3 tests got "%s" "%s" "%s"`, str0, str1, str2)
			}
		})
	})

}

var coStepInitiate = Step{
	description: "User initiates the routine, and server replies with protocol version.",
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
}

var coStepValidPk = func(pk model.PublicKey, msgToSign ...string) Step {
	return Step{
		description: "User provides a valid public key, and server replies with a message for the user to sign using their private key",
		input: model.RoutineInput{
			MsgType: model.RoutineMsgType_UsrMsg,
			Msg: `{
				"publicKey": "` + (string)(pk) + `"
			}`,
		},
		outputs: []ExpectedOutput{
			{
				ro: model.RoutineOutput{
					Msgs: []string{comeOnlineSignThisResponseSchema(msgToSign...)},
				},
			},
		},
	}
}

var coStepValidSignature = func(signature string) Step {
	return Step{
		description: "User replies with the correct signature for the message, and server welcomes the user.",
		input: model.RoutineInput{
			MsgType: model.RoutineMsgType_UsrMsg,
			Msg: `{
				"signature": "` + signature + `"
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
	}
}

var coStepClientCancel = Step{
	description: "Client cancel",
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
}

var coStepClientClose = Step{
	description: "Client close",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_ClientClose,
	},
	// expect no output
}

var coStepTimeout = Step{
	description: "Client timeout",
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
}

var coStepInvalidSignature = func(signatureMessage string, errorMessage ...string) Step {
	return Step{
		description: "Client sends an invalid signature, server cancels the transaction and replies with an error message.",
		input: model.RoutineInput{
			MsgType: model.RoutineMsgType_UsrMsg,
			Msg:     signatureMessage,
		},
		outputs: []ExpectedOutput{
			{
				ro: model.RoutineOutput{
					Done: true,
					Msgs: []string{errorSchemaString(errorMessage...)},
				},
			},
		},
	}
}

var coStepBadPublicKey = func(publicKeyMessage string, errorMessage ...string) Step {
	return Step{
		description: "Client sends a bad public key, server cancels the transaction and replies with an error message.",
		input: model.RoutineInput{
			MsgType: model.RoutineMsgType_UsrMsg,
			Msg:     publicKeyMessage,
		},
		outputs: []ExpectedOutput{
			{
				ro: model.RoutineOutput{
					Msgs: []string{errorSchemaString(errorMessage...)},
					Done: true,
				},
			},
		},
	}
}

const testMessage = "This is a test message used to verify the public key. Usually, it would consist of random characters. It is sent to the user, who hashes and signs it with their private key. The signature is sent back to this server, which verifies the signature against the public key."

// testMessage signed with publicKey0
const testPk0Signature = "jIX/9ZHy6UuGZzywconx5rSV77yGugYg2M40ROilWS/zo3qnlau2Zn2p045ZYvKDH98LrMm8vJOmdmWBCkY0Bg=="

// non-random message generator for mocking.
type fixedMessageGenerator struct {
	msg string
}

func (g fixedMessageGenerator) GetMessage() (string, error) {
	return g.msg, nil
}
