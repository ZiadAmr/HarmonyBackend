package routines

import (
	"harmony/backend/model"
	"strconv"
	"testing"
	"time"
)

const frExpectedTimeoutDuration = 10 * time.Second

func TestFriendRequest(t *testing.T) {

	t.Run("Valid inputs", func(t *testing.T) {

		t.Run("Friend is offline", func(t *testing.T) {
			test := []Step{
				frStepInitiateOffline,
			}

			client := &model.Client{}
			client.SetPublicKey(&publicKey0)
			hub := model.NewHub()
			hub.AddClient(*client.GetPublicKey(), client)
			fr := newFriendRequest(client, hub)

			testRunner(t, fr, test)

		})

		t.Run("Friend is online", func(t *testing.T) {

			statuses := []string{"accept", "reject", "pending"}

			for _, status := range statuses {
				t.Run(status, func(t *testing.T) {

					test := []Step{
						frStepInitiateOnline,
						frResponseFromB(status),
					}

					clientA := &model.Client{}
					clientA.SetPublicKey(&publicKey0)
					clientB := &model.Client{}
					clientB.SetPublicKey(&publicKey1)
					hub := model.NewHub()
					hub.AddClient(*clientA.GetPublicKey(), clientA)
					hub.AddClient(*clientB.GetPublicKey(), clientB)

					fr := newFriendRequest(clientA, hub)

					testRunner(t, fr, test)
				})
			}
		})

	})

	t.Run("Invalid inputs", func(t *testing.T) {
		t.Run("User has not provided their public key", func(t *testing.T) {

			test := []Step{
				{
					description: "A sends a request without having provided their public key",
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Pk:      nil,
						Msg:     frStepInitiateOnline.input.Msg,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Pk:   nil,
								Msgs: []string{errorSchemaString("You have not provided a public key")},
								Done: true,
							},
						},
					},
				},
			}

			client := &model.Client{}
			hub := model.NewHub()
			fr := newFriendRequest(client, hub)

			testRunner(t, fr, test)
		})

		t.Run("User attempts to send a friend request to themself", func(t *testing.T) {
			test := []Step{
				{
					description: "User sends a friend request to themself",
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Pk:      &publicKey0,
						Msg: `{
							"initiate": "sendFriendRequest",
							"key": "` + (string)(publicKey0) + `"
						}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Pk:   &publicKey0,
								Msgs: []string{errorSchemaString("Sending a friend request to yourself is not allowed")},
								Done: true,
							},
						},
					},
				},
			}
			client := &model.Client{}
			client.SetPublicKey(&publicKey0)
			hub := model.NewHub()
			hub.AddClient(*client.GetPublicKey(), client)
			fr := newFriendRequest(client, hub)

			testRunner(t, fr, test)
		})

		// tests where both are signed in
		tests := []struct {
			description  string
			prefaceSteps []Step
			cases        []Step
		}{
			{
				description:  "A sends a bad initial message",
				prefaceSteps: []Step{},
				cases: []Step{
					{
						description: "No key",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `{"initiate": "sendFriendRequest"}`,
						},
						outputs: outputPkAError,
					},

					{
						description: "Key in wrong format",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `{"initiate": "sendFriendRequest", "key":"4"}`,
						},
						outputs: outputPkAError,
					},

					{
						description: "Invalid JSON",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `)`,
						},
						outputs: outputPkAError,
					},

					{
						description: "Extra properties",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `{"initiate": "sendFriendRequest", "key":"` + (string)(publicKey1) + `", "extraProperty!":{}}`,
						},
						outputs: outputPkAError,
					},
				},
			},
			{
				description: "A has sent request to B",
				prefaceSteps: []Step{
					frStepInitiateOnline,
				},
				cases: []Step{
					stepPkADisconnect,
					stepPkBDisconnect,
					stepPkBTimeout,
					stepPkACancel,
					stepPkBCancel,
					{
						description: "B sends no forward property",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey1,
							Msg:     `{}`,
						},
						outputs: outputPkBErrorToBoth,
					},
					{
						description: "B sends additional properties",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey1,
							Msg:     `"{forward":{"type":"reject"},"what": true}`,
						},
						outputs: outputPkBErrorToBoth,
					},
					{
						description: "B sends malformed JSON",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey1,
							Msg:     `{`,
						},
						outputs: outputPkBErrorToBoth,
					},
					{
						description: "B sends invalid response (not reject, accept, or pending)",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey1,
							Msg: `{
								"forward": {
									"type": "\""
								}
							}`,
						},
						outputs: outputPkBErrorToBoth,
					},
					{
						description: "A sends message out of order",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     "boo!",
						},
						outputs: outputPkAErrorToBoth,
					},
				},
			},
		}

		for _, test := range tests {

			for j, testCase := range test.cases {

				t.Run(test.description+"-"+strconv.Itoa(j), func(t *testing.T) {

					clientA := &model.Client{}
					clientA.SetPublicKey(&publicKey0)
					clientB := &model.Client{}
					clientB.SetPublicKey(&publicKey1)
					hub := model.NewHub()
					hub.AddClient(publicKey0, clientA)
					hub.AddClient(publicKey1, clientB)
					fr := newFriendRequest(clientA, hub)

					testRunner(t, fr, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})
				})

			}
		}

	})

}

var frStepInitiateOffline = Step{
	description: "A sends friend request",
	input: model.RoutineInput{
		Pk:      &publicKey0,
		MsgType: model.RoutineMsgType_UsrMsg,
		Msg: `{
			"initiate": "sendFriendRequest",
			"key": "` + (string)(publicKey1) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk: &publicKey0,
				// reuse ectp cos it's the same
				Msgs: []string{ectpSchemaOfflineToA},
				Done: true,
			},
		},
	},
}

var frStepInitiateOnline = Step{
	description: "A sends a request and server sends a message to B",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey0,
		Msg: `{
			"initiate": "sendFriendRequest",
			"key": "` + (string)(publicKey1) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &publicKey1,
				Msgs:            []string{frSchemaInitiateToB((string)(publicKey0))},
				TimeoutEnabled:  true,
				TimeoutDuration: frExpectedTimeoutDuration,
			},
		},
	},
}

func frResponseFromB(status string) Step {
	return Step{
		description: "B responds with status " + status,
		input: model.RoutineInput{
			MsgType: model.RoutineMsgType_UsrMsg,
			Pk:      &publicKey1,
			Msg: `{
				"forward": {
					"type": "` + status + `"
				}
			}`,
		},
		outputs: []ExpectedOutput{
			{
				ro: model.RoutineOutput{
					Pk:   &publicKey1,
					Msgs: []string{schemaBareTerminate},
					Done: true,
				},
			},
			{
				ro: model.RoutineOutput{
					Pk:   &publicKey0,
					Msgs: []string{frForwardToA(status)},
					Done: true,
				},
			},
		},
	}
}

func frSchemaInitiateToB(pkb32 string) string {
	return `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"initiate": {
				"const":"receiveFriendRequest"
			},
			"key": {
				"const": "` + pkb32 + `"
			}
		},
		"required": ["initiate", "key"],
		"additionalProperties": false
	}`
}

func frForwardToA(status string) string {
	return `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"peerStatus": {
				"const":"online"
			},
			"forwarded": {
				"properties": {
					"type": {
						"const":"` + status + `"
					}
				},
				"required": ["type"],
				"additionalProperties": false  
			},
			"terminate": {
				"const":"done"
			}
		},
		"required": ["peerStatus", "forwarded", "terminate"],
		"additionalProperties": false
	}`
}
