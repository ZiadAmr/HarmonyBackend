package routines

import (
	"encoding/hex"
	"harmony/backend/model"
	"strconv"
	"testing"
	"time"
)

const ectpExpectedTimeoutDuration = 20 * time.Second

func TestEstablishConnectionToPeer(t *testing.T) {

	t.Run("Valid inputs", func(t *testing.T) {

		t.Run("Friend is offline", func(t *testing.T) {

			test := []Step{
				ectpStepInitiateOffline,
			}

			client := &model.Client{}
			client.SetPublicKey(&pkA)
			hub := model.NewHub()
			ectp := newEstablishConnectionToPeer(client, hub)

			testRunner(t, ectp, test)
		})

		t.Run("friend rejects", func(t *testing.T) {
			test := []Step{
				ectpStepInitiateOnline,
				ectpStepReject,
			}

			clientA := &model.Client{}
			clientA.SetPublicKey(&pkA)
			clientB := &model.Client{}
			clientB.SetPublicKey(&pkB)
			hub := model.NewHub()
			hub.AddClient(pkA, clientA)
			hub.AddClient(pkB, clientB)
			ectp := newEstablishConnectionToPeer(clientA, hub)

			testRunner(t, ectp, test)
		})

		t.Run("clients connect", func(t *testing.T) {

			tests := [][]Step{
				{
					ectpStepInitiateOnline,
					ectpStepAcceptAndOffer,
					ectpStepAnswer,
					ectpStepIceAToB,
					ectpStepIceBtoA,
					ectpStepFinalIceA,
					ectpStepFinalIceBTerminate,
				},
				{
					ectpStepInitiateOnline,
					ectpStepAcceptAndOffer,
					ectpStepAnswer,
					ectpStepIceBtoA, // ice candidates in different order
					ectpStepIceAToB,
					ectpStepFinalIceB, // terminates in different order
					ectpStepIceAToB,   // ice candidate sent after the other client has finished
					ectpStepFinalIceATerminate,
				},
			}

			for i, test := range tests {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					clientA := &model.Client{}
					clientA.SetPublicKey(&pkA)
					clientB := &model.Client{}
					clientB.SetPublicKey(&pkB)
					hub := model.NewHub()
					hub.AddClient(pkA, clientA)
					hub.AddClient(pkB, clientB)
					ectp := newEstablishConnectionToPeer(clientA, hub)

					testRunner(t, ectp, test)
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
						Msg: `{
							"initiate": "sendConnectionRequest",
							"key": "` + hex.EncodeToString(pkB[:]) + `"
						}`,
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
			ectp := newEstablishConnectionToPeer(client, hub)

			testRunner(t, ectp, test)
		})

		t.Run("User tries to connect to themself", func(t *testing.T) {
			test := []Step{
				{
					description: "A sends a connection request to A",
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Pk:      &pkA,
						Msg: `{
							"initiate": "sendConnectionRequest",
							"key": "` + hex.EncodeToString(pkA[:]) + `"
						}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Pk:   &pkA,
								Msgs: []string{errorSchemaString("Connecting to yourself is not allowed")},
								Done: true,
							},
						},
					},
				},
			}

			client := &model.Client{}
			client.SetPublicKey(&pkA)
			hub := model.NewHub()
			hub.AddClient(pkA, client)
			ectp := newEstablishConnectionToPeer(client, hub)

			testRunner(t, ectp, test)
		})

		t.Run("Friend is offline", func(t *testing.T) {
			tests := [][]Step{
				{
					{
						description: "No key",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &pkA,
							Msg:     `{"initiate": "sendConnectionRequest"}`,
						},
						outputs: outputPkAError,
					},
				},
				{
					{
						description: "Key in wrong format",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &pkA,
							Msg:     `{"initiate": "sendConnectionRequest", "key":"4"}`,
						},
						outputs: outputPkAError,
					},
				},
				{
					{
						description: "Invalid JSON",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &pkA,
							Msg:     `)`,
						},
						outputs: outputPkAError,
					},
				},
				{
					{
						description: "Extra properties",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &pkA,
							Msg:     `{"initiate": "sendConnectionRequest", "key":"` + hex.EncodeToString(pkB[:]) + `", "extraProperty!":{}}`,
						},
						outputs: outputPkAError,
					},
				},
			}

			for i, test := range tests {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					client := &model.Client{}
					client.SetPublicKey(&pkA)
					hub := model.NewHub()
					ectp := newEstablishConnectionToPeer(client, hub)

					testRunner(t, ectp, test)
				})
			}
		})
		t.Run("Friend is online", func(t *testing.T) {

			tests := []struct {
				description  string
				prefaceSteps []Step
				cases        []Step
			}{
				{
					description: "A initiates; server has send msg to B",
					prefaceSteps: []Step{
						ectpStepInitiateOnline,
					},
					cases: []Step{
						stepPkADisconnect,
						stepPkBDisconnect,
						stepPkBTimeout,
						stepPkACancel,
						stepPkBCancel,
						{
							description: "B sends bad input",
							input: model.RoutineInput{
								MsgType: model.RoutineMsgType_UsrMsg,
								Pk:      &pkB,
								Msg:     "lol",
							},
							outputs: outputPkBErrorToBoth,
						},
						{
							description: "A sends a message out of order",
							input:       ectpStepAnswer.input,
							outputs:     outputPkAErrorToBoth,
						},
					},
				},
				{
					description: "B has sent sdp offer to A",
					prefaceSteps: []Step{
						ectpStepInitiateOnline,
						ectpStepAcceptAndOffer,
					},
					cases: []Step{
						stepPkADisconnect,
						stepPkBDisconnect,
						stepPkATimeout,
						stepPkACancel,
						stepPkBCancel,
						{
							description: "A sends bad input",
							input: model.RoutineInput{
								MsgType: model.RoutineMsgType_UsrMsg,
								Pk:      &pkA,
								Msg:     "xd",
							},
							outputs: outputPkAErrorToBoth,
						},
						{
							description: "B sends a message out of order",
							input:       ectpStepIceBtoA.input,
							outputs:     outputPkBErrorToBoth,
						},
					},
				},
				{
					description: "Both have exchanged SDPs, now are exchanging ICE candidates",
					prefaceSteps: []Step{
						ectpStepInitiateOnline,
						ectpStepAcceptAndOffer,
						ectpStepAnswer,
					},
					cases: []Step{
						stepPkADisconnect,
						stepPkBDisconnect,
						stepPkATimeout,
						stepPkBTimeout,
						stepPkACancel,
						stepPkBCancel,
						{
							description: "A sends bad input",
							input: model.RoutineInput{
								MsgType: model.RoutineMsgType_UsrMsg,
								Pk:      &pkA,
								Msg:     "lol",
							},
							outputs: outputPkAErrorToBoth,
						},
						{
							description: "B sends bad input",
							input: model.RoutineInput{
								MsgType: model.RoutineMsgType_UsrMsg,
								Pk:      &pkB,
								Msg:     "lol",
							},
							outputs: outputPkBErrorToBoth,
						},
					},
				},
				{
					description: "A has finished sending ICE candidates (but not B)",
					prefaceSteps: []Step{
						ectpStepInitiateOnline,
						ectpStepAcceptAndOffer,
						ectpStepAnswer,
						ectpStepIceAToB,
						ectpStepIceBtoA,
						ectpStepFinalIceA,
					},
					cases: []Step{
						stepPkADisconnect,
						stepPkBDisconnect,
						stepPkBTimeout,
						stepPkACancel,
						stepPkBCancel,
						{
							description: "A sends another ice candidate after the final once",
							input:       ectpStepIceAToB.input,
							outputs:     outputPkAErrorToBoth,
						},
					},
				},
				{
					description: "B has finished sending ICE candidates (but not A)",
					prefaceSteps: []Step{
						ectpStepInitiateOnline,
						ectpStepAcceptAndOffer,
						ectpStepAnswer,
						ectpStepIceAToB,
						ectpStepIceBtoA,
						ectpStepFinalIceB,
					},
					cases: []Step{
						stepPkADisconnect,
						stepPkBDisconnect,
						stepPkATimeout,
						stepPkACancel,
						stepPkBCancel,
						{
							description: "B sends another message candidate after the final once",
							input:       ectpStepIceBtoA.input,
							outputs:     outputPkBErrorToBoth,
						},
					},
				},
			}

			for _, test := range tests {

				for j, testCase := range test.cases {

					t.Run(test.description+"-"+strconv.Itoa(j), func(t *testing.T) {

						clientA := &model.Client{}
						clientA.SetPublicKey(&pkA)
						clientB := &model.Client{}
						clientB.SetPublicKey(&pkB)
						hub := model.NewHub()
						hub.AddClient(pkA, clientA)
						hub.AddClient(pkB, clientB)
						ectp := newEstablishConnectionToPeer(clientA, hub)

						testRunner(t, ectp, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})
					})

				}
			}
		})
	})

}

const ectpSchemaOfflineToA = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"peerStatus": {
			"const":"offline"
		},
		"forwarded": {
			"const": null 
		},
		"terminate": {
			"const":"done"
		}
	},
	"required": ["peerStatus", "forwarded", "terminate"],
	"additionalProperties": false
}`

var ectpSchemaInitiateToB = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"initiate": {
			"const":"receiveConnectionRequest"
		},
		"key": {
			"type":"string",
			"pattern": "^[0123456789abcdef]{` + strconv.Itoa(model.KEYLEN*2) + `}$"
		}
	},
	"required": ["initiate", "key"],
	"additionalProperties": false
}`

const schemaBareTerminate = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"terminate": {
			"const":"done"
		}
	},
	"required": ["terminate"],
	"additionalProperties": false
}
`

const ectpSchemaRejectToA = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"peerStatus": {
			"const":"online"
		},
		"forwarded": {
			"properties": {
				"type": {
					"const":"reject"
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

func ectpSchemaAcceptAndOfferToA(sdp string) string {
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
						"const":"acceptAndOffer"
					},
					"payload": {
						"properties": {
							"type": {
								"const":"offer" 
							},
							"sdp": {
								"const":"` + sdp + `"
							}
						},
						"required": ["type", "sdp"],
						"additionalProperties": false  
					}
				},
				"required": ["type", "payload"],
				"additionalProperties": false  
			}
		},
		"required": ["peerStatus", "forwarded"],
		"additionalProperties": false
	}`
}

func ectpSchemaAnswerToB(sdp string) string {
	return `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"forwarded": {
				"properties": {
					"type": {
						"const":"answer"
					},
					"payload": {
						"properties": {
							"type": {
								"const":"answer" 
							},
							"sdp": {
								"const":"` + sdp + `"
							}
						},
						"required": ["type", "sdp"],
						"additionalProperties": false  
					}
				},
				"required": ["type", "payload"],
				"additionalProperties": false  
			}
		},
		"required": ["forwarded"],
		"additionalProperties": false
	}`
}

func ectpSchemaIceCandidate(payload string) string {
	return `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"forwarded": {
				"properties": {
					"type": {
						"const":"ICECandidate"
					},
					"payload": {
						"const":` + payload + `
					}
				},
				"required": ["type", "payload"],
				"additionalProperties": false  
			}
		},
		"required": ["forwarded"],
		"additionalProperties": false
	}`
}

var pkA = (model.PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))
var pkB = (model.PublicKey)([]byte("\x82\x30\xa6\x9f\x8a\x09\xc3\x84\xeb\xf8\xcc\xfd\xe7\x7a\x65\x06\xcf\x1c\xc0\x53\x6b\xa9\x6d\x6f\xde\x97\x13\xbd\x13\x63\xe0\x12\x17\x09\x25\xab\x5e\x0b\x21\x0e\xe0\x87\xbf\xd2\x15\x45\x87\xe7\x23\x5d\x02\xbe\xc1\x65\x48\xa2\xe2\x29\xc0\xee\xcc\x24\x14\x9d"))

// TODO>>
const sdpOffer = "replace this with an actual offer"
const sdpAnswer = `replace this with an actual answer`
const ICECandidate0 = `{
	"candidate":"an actual ice candidate",
	"sdpMLineIndex":0,
	"sdpMid":"...",
	"usernameFragment":"..."
}`
const ICECandidate1 = `{
	"candidate":"another actual ice candidate",
	"sdpMLineIndex":0
}`
const ICECandidateDone = `{
	"candidate":"",
	"sdpMLineIndex":0,
	"sdpMid":"...",
	"usernameFragment":"..."
}`

var ectpStepInitiateOnline = Step{
	description: "A sends a request and server sends a message to B",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkA,
		Msg: `{
			"initiate": "sendConnectionRequest",
			"key": "` + hex.EncodeToString(pkB[:]) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkB,
				Msgs:            []string{ectpSchemaInitiateToB},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepInitiateOffline = Step{
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkA,
		Msg: `{
			"initiate": "sendConnectionRequest",
			"key": "` + hex.EncodeToString(pkB[:]) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &pkA,
				Msgs: []string{ectpSchemaOfflineToA},
				Done: true,
			},
		},
	},
}

var ectpStepAcceptAndOffer = Step{
	description: "B sends an offer and server passes it to A",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkB,
		Msg: `{
			"forward": {
				"type": "acceptAndOffer",
				"payload": {
					"type": "offer",
					"sdp": "` + sdpOffer + `"
				}
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkA,
				Msgs:            []string{ectpSchemaAcceptAndOfferToA(sdpOffer)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepReject = Step{
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkB,
		Msg: `{
			"forward": {
				"type": "reject"
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &pkB,
				Msgs: []string{schemaBareTerminate},
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &pkA,
				Msgs: []string{ectpSchemaRejectToA},
				Done: true,
			},
		},
	},
}

var ectpStepAnswer = Step{
	description: "A sends an answer and server passes it to B",
	input: model.RoutineInput{
		Pk:      &pkA,
		MsgType: model.RoutineMsgType_UsrMsg,
		Msg: `{
			"forward": {
				"type": "answer",
				"payload": {
					"type": "answer",
					"sdp": "` + sdpAnswer + `"
				}
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkB,
				Msgs:            []string{ectpSchemaAnswerToB(sdpAnswer)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepIceAToB = Step{
	description: "A sends an ICE candidate, server forwards it to B",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkA,
		Msg: `{
			"forward": {
				"type": "ICECandidate",
				"payload": ` + ICECandidate0 + `
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkB,
				Msgs:            []string{ectpSchemaIceCandidate(ICECandidate0)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepIceBtoA = Step{
	description: "B sends an ICE candidate, server forwards it to A",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkB,
		Msg: `{
			"forward": {
				"type": "ICECandidate",
				"payload": ` + ICECandidate1 + `
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkA,
				Msgs:            []string{ectpSchemaIceCandidate(ICECandidate1)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepFinalIceA = Step{
	description: "A sends an empty ICE candidate to denote end of ice candidates, server passes it to B",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkA,
		Msg: `{
			"forward": {
				"type": "ICECandidate",
				"payload": ` + ICECandidateDone + `
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkB,
				Msgs:            []string{ectpSchemaIceCandidate(ICECandidateDone)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepFinalIceATerminate = Step{
	description: "A sends an empty ICE candidate to denote end of ice candidates, server passes it to B and terminates both transaction sockets",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkA,
		Msg: `{
			"forward": {
				"type": "ICECandidate",
				"payload": ` + ICECandidateDone + `
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			// both clients have finished sending messages, send terminate:done to both
			ro: model.RoutineOutput{
				Pk:   &pkB,
				Msgs: []string{ectpSchemaIceCandidate(ICECandidateDone), schemaBareTerminate},
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &pkA,
				Msgs: []string{schemaBareTerminate},
				Done: true,
			},
		},
	},
}

var ectpStepFinalIceB = Step{
	description: "B sends an empty ICE candidate to denote end of ice candidates, server passes it to A",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkB,
		Msg: `{
			"forward": {
				"type": "ICECandidate",
				"payload": ` + ICECandidateDone + `
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &pkA,
				Msgs:            []string{ectpSchemaIceCandidate(ICECandidateDone)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepFinalIceBTerminate = Step{
	description: "B sends an empty ICE candidate to denote end of ice candidates, server passes it to A and terminates both transaction sockets",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkB,
		Msg: `{
			"forward": {
				"type": "ICECandidate",
				"payload": ` + ICECandidateDone + `
			}
		}`,
	},
	outputs: []ExpectedOutput{
		{
			// both clients have finished sending messages, send terminate:done to both
			ro: model.RoutineOutput{
				Pk:   &pkA,
				Msgs: []string{ectpSchemaIceCandidate(ICECandidateDone), schemaBareTerminate},
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &pkB,
				Msgs: []string{schemaBareTerminate},
				Done: true,
			},
		},
	},
}

var stepPkADisconnect = Step{
	description: "A disconnects",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_ClientClose,
		Pk:      &pkA,
	},
	outputs: outputPkADisconnectedToB,
}

var stepPkBDisconnect = Step{
	description: "B disconnects",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_ClientClose,
		Pk:      &pkB,
	},
	outputs: outputPkBDisconnectedToA,
}

var stepPkACancel = Step{
	description: "A cancels",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkA,
		Msg:     `{"terminate":"cancel"}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &pkA,
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &pkB,
				Msgs: []string{errorSchemaString("Peer cancelled the transaction")},
				Done: true,
			},
		},
	},
}
var stepPkBCancel = Step{
	description: "B cancels",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &pkB,
		Msg:     `{"terminate":"cancel"}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &pkB,
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &pkA,
				Msgs: []string{errorSchemaString("Peer cancelled the transaction")},
				Done: true,
			},
		},
	},
}

var stepPkATimeout = Step{
	description: "A times out",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_Timeout,
		Pk:      &pkA,
	},
	outputs: outputPkATimeoutToBoth,
}

var stepPkBTimeout = Step{
	description: "B times out",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_Timeout,
		Pk:      &pkB,
	},
	outputs: outputPkBTimeoutToBoth,
}

var outputPkAError = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkA,
			Msgs: []string{errorSchemaString()},
			Done: true,
		},
	},
}

var outputPkAErrorToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkA,
			Msgs: []string{errorSchemaString()},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{
			Pk:   &pkB,
			Msgs: []string{errorSchemaString("Peer sent a malformed message")},
			Done: true,
		},
	},
}

var outputPkATimeoutToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkA,
			Msgs: []string{errorSchemaString("Timeout")},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{

			Pk:   &pkB,
			Msgs: []string{errorSchemaString("Peer timed out")},
			Done: true,
		},
	},
}

var outputPkBErrorToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkB,
			Msgs: []string{errorSchemaString()},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{

			Pk:   &pkA,
			Msgs: []string{errorSchemaString("Peer sent a malformed message")},
			Done: true,
		},
	},
}

var outputPkBTimeoutToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkB,
			Msgs: []string{errorSchemaString("Timeout")},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{

			Pk:   &pkA,
			Msgs: []string{errorSchemaString("Peer timed out")},
			Done: true,
		},
	},
}

var outputPkADisconnectedToB = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkB,
			Msgs: []string{errorSchemaString("Peer disconnected")},
			Done: true,
		},
	},
}

var outputPkBDisconnectedToA = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &pkA,
			Msgs: []string{errorSchemaString("Peer disconnected")},
			Done: true,
		},
	},
}
