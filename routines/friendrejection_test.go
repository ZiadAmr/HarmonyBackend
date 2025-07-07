package routines

import (
	"harmony/backend/model"
	"testing"
)

func TestFriendRejection(t *testing.T) {

	t.Run("Valid inputs", func(t *testing.T) {

		t.Run("Peer is online", func(t *testing.T) {
			test := []Step{
				frejStepOnline,
			}

			clientA := &model.Client{}
			clientA.SetPublicKey(&publicKey0)
			clientB := &model.Client{}
			clientB.SetPublicKey(&publicKey1)
			hub := model.NewHub()
			hub.AddClient(*clientA.GetPublicKey(), clientA)
			hub.AddClient(*clientB.GetPublicKey(), clientB)
			fr := newFriendRejection(clientA, hub)

			testRunner(t, fr, test)

		})

		t.Run("Peer is offline", func(t *testing.T) {

			test := []Step{
				frejStepOffline,
			}

			clientA := &model.Client{}
			clientA.SetPublicKey(&publicKey0)
			hub := model.NewHub()
			hub.AddClient(*clientA.GetPublicKey(), clientA)
			fr := newFriendRejection(clientA, hub)

			testRunner(t, fr, test)
		})
	})

	t.Run("Invalid inputs", func(t *testing.T) {
		t.Run("User has not provided their public key", func(t *testing.T) {

			test := []Step{
				{
					description: "A sends a friend rejection without having provided their public key",
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Pk:      nil,
						Msg:     frejStepOnline.input.Msg,
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

			clientA := &model.Client{}
			clientB := &model.Client{}
			clientB.SetPublicKey(&publicKey1)
			hub := model.NewHub()
			hub.AddClient(*clientB.GetPublicKey(), clientB)

			fr := newFriendRejection(clientA, hub)

			testRunner(t, fr, test)
		})

		t.Run("User sends a message to themself", func(t *testing.T) {
			test := []Step{
				{
					description: "User sends a rejection to themself",
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Pk:      &publicKey0,
						Msg: `{
							"initiate": "sendFriendRejection",
							"key": "` + (string)(publicKey0) + `"
						}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Pk:   &publicKey0,
								Msgs: []string{errorSchemaString("You can't reject yourself")},
								Done: true,
							},
						},
					},
				},
			}

			clientA := &model.Client{}
			clientA.SetPublicKey(&publicKey0)
			hub := model.NewHub()
			hub.AddClient(*clientA.GetPublicKey(), clientA)

			fr := newFriendRejection(clientA, hub)

			testRunner(t, fr, test)
		})

		tests := []Step{
			{
				description: "No key",
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Pk:      &publicKey0,
					Msg:     `{"initiate": "sendFriendRejection"}`,
				},
				outputs: outputPkAError,
			},

			{
				description: "Key in wrong format",
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Pk:      &publicKey0,
					Msg:     `{"initiate": "sendFriendRejection", "key":"4"}`,
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
					Msg:     `{"initiate": "sendFriendRejection", "key":"` + (string)(publicKey1) + `", "extraProperty!":{}}`,
				},
				outputs: outputPkAError,
			},
		}

		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				clientA := &model.Client{}
				clientA.SetPublicKey(&publicKey0)
				clientB := &model.Client{}
				clientB.SetPublicKey(&publicKey1)
				hub := model.NewHub()
				hub.AddClient(*clientA.GetPublicKey(), clientA)
				hub.AddClient(*clientB.GetPublicKey(), clientB)

				fr := newFriendRejection(clientA, hub)

				testRunner(t, fr, []Step{test})
			})
		}
	})
}

var frejStepOnline = Step{
	description: "Send friend rejection to online friend",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey0,
		Msg: `{
			"initiate": "sendFriendRejection",
			"key": "` + (string)(publicKey1) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Msgs: []string{frejSchemaOnlineToA},
				Done: true,
			},
		}, {
			ro: model.RoutineOutput{
				Pk:   &publicKey1,
				Msgs: []string{frejSchemaOnlineToB},
				Done: true,
			},
		},
	},
}

var frejStepOffline = Step{
	description: "Send friend rejection to offline friend",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey0,
		Msg: `{
			"initiate": "sendFriendRejection",
			"key": "` + (string)(publicKey1) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Msgs: []string{frejSchemaOfflineToA},
				Done: true,
			},
		},
	},
}

const frejSchemaOfflineToA = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"peerStatus": {
			"const": "offline"
		},
		"terminate": {
			"const": "done"
		}
	},
	"additionalProperties": false,
	"required": ["peerStatus", "terminate"]
}`

const frejSchemaOnlineToA = `{
"$schema": "https://json-schema.org/draft/2020-12/schema",
"type": "object",
"properties": {
	"peerStatus": {
		"const": "online"
	},
	"terminate": {
		"const": "done"
	}
},
"additionalProperties": false,
"required": ["peerStatus", "terminate"]
}`

var frejSchemaOnlineToB = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"initiate": {
			"const": "receiveFriendRejection"
		},
		"terminate": {
			"const": "done"
		},
		"key": {
			"const": "` + (string)(publicKey0) + `"
		}
	},
	"additionalProperties": false,
	"required": ["initiate", "terminate", "key"]
}`
