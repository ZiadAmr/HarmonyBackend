package routines

import (
	"bytes"
	"harmony/backend/model"
	"log/slog"
	"strconv"
	"testing"
	"time"
)

const establishConnectionToPeerRoutineName = "establishConnectionToPeer"

var establishConnectionToPeerLoggerAttrs = toAnySlice("ip", ip0, "pk", string(publicKey0), "routine", establishConnectionToPeerRoutineName, "tsid", tsid1)

const ectpExpectedTimeoutDuration = 20 * time.Second
const maxIceCandidates = 20

func TestEstablishConnectionToPeer(t *testing.T) {

	t.Run("Valid inputs", func(t *testing.T) {

		t.Run("Friend is offline", func(t *testing.T) {

			test := []Step{
				ectpStepInitiateOffline,
			}

			client := &model.Client{IpAddr: ip0}
			client.SetPublicKey(&publicKey0)
			hub := model.NewHub(testAllowedHostnames)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...)
			ectp := newEstablishConnectionToPeer(client, hub, mockLogger)

			testRunner(t, ectp, &logOutput, test)
		})

		t.Run("friend rejects", func(t *testing.T) {
			test := []Step{
				ectpStepInitiateOnline,
				ectpStepReject,
			}

			clientA := &model.Client{IpAddr: ip0}
			clientA.SetPublicKey(&publicKey0)
			clientB := &model.Client{IpAddr: ip1}
			clientB.SetPublicKey(&publicKey1)
			hub := model.NewHub(testAllowedHostnames)
			hub.AddClient(publicKey0, clientA)
			hub.AddClient(publicKey1, clientB)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...)
			ectp := newEstablishConnectionToPeer(clientA, hub, mockLogger)

			testRunner(t, ectp, &logOutput, test)
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
					clientA := &model.Client{IpAddr: ip0}
					clientA.SetPublicKey(&publicKey0)
					clientB := &model.Client{IpAddr: ip0}
					clientB.SetPublicKey(&publicKey1)
					hub := model.NewHub(testAllowedHostnames)
					hub.AddClient(publicKey0, clientA)
					hub.AddClient(publicKey1, clientB)
					var logOutput bytes.Buffer
					mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...)
					ectp := newEstablishConnectionToPeer(clientA, hub, mockLogger)

					testRunner(t, ectp, &logOutput, test)
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
							"key": "` + (string)(publicKey1) + `"
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
					logs: []ExpectedLog{
						{
							level: "INFO",
							kind:  ROUTINE_FAIL,
							client: &ClientLogAttributes{
								ip: ip0,
								pk: "nil",
							},
							transaction: &TransactionLogAttributes{
								routine: establishConnectionToPeerRoutineName,
								tsid:    tsid1,
							},
						},
					},
				},
			}

			client := &model.Client{IpAddr: ip0}
			hub := model.NewHub(testAllowedHostnames)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...).With("pk", "nil")
			ectp := newEstablishConnectionToPeer(client, hub, mockLogger)

			testRunner(t, ectp, &logOutput, test)
		})

		t.Run("User tries to connect to themself", func(t *testing.T) {
			test := []Step{
				{
					description: "A sends a connection request to A",
					input: model.RoutineInput{
						MsgType: model.RoutineMsgType_UsrMsg,
						Pk:      &publicKey0,
						Msg: `{
							"initiate": "sendConnectionRequest",
							"key": "` + (string)(publicKey0) + `"
						}`,
					},
					outputs: []ExpectedOutput{
						{
							ro: model.RoutineOutput{
								Pk:   &publicKey0,
								Msgs: []string{errorSchemaString("Connecting to yourself is not allowed")},
								Done: true,
							},
						},
					},
					logs: []ExpectedLog{
						ectpLog("INFO", ROUTINE_FAIL, "send to self"),
					},
				},
			}

			client := &model.Client{IpAddr: ip0}
			client.SetPublicKey(&publicKey0)
			hub := model.NewHub(testAllowedHostnames)
			hub.AddClient(publicKey0, client)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...)
			ectp := newEstablishConnectionToPeer(client, hub, mockLogger)

			testRunner(t, ectp, &logOutput, test)
		})

		t.Run("Friend is offline", func(t *testing.T) {
			tests := [][]Step{
				{
					{
						description: "No key",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `{"initiate": "sendConnectionRequest"}`,
						},
						outputs: outputPkAError,
						logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
					},
				},
				{
					{
						description: "Key in wrong format",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `{"initiate": "sendConnectionRequest", "key":"4"}`,
						},
						outputs: outputPkAError,
						logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
					},
				},
				{
					{
						description: "Invalid JSON",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `)`,
						},
						outputs: outputPkAError,
						logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
					},
				},
				{
					{
						description: "Extra properties",
						input: model.RoutineInput{
							MsgType: model.RoutineMsgType_UsrMsg,
							Pk:      &publicKey0,
							Msg:     `{"initiate": "sendConnectionRequest", "key":"` + (string)(publicKey1) + `", "extraProperty!":{}}`,
						},
						outputs: outputPkAError,
						logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
					},
				},
			}

			for i, test := range tests {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					client := &model.Client{}
					client.SetPublicKey(&publicKey0)
					hub := model.NewHub(testAllowedHostnames)
					var logOutput bytes.Buffer
					mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...)
					ectp := newEstablishConnectionToPeer(client, hub, mockLogger)
					testRunner(t, ectp, &logOutput, test)
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
					description: "A initiates; server has sent msg to B",
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
								Pk:      &publicKey1,
								Msg:     "lol",
							},
							outputs: outputPkBErrorToBoth,
							logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
						},
						{
							description: "A sends a message out of order",
							input:       ectpStepAnswer.input,
							outputs:     outputPkAErrorToBoth,
							logs:        []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
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
								Pk:      &publicKey0,
								Msg:     "xd",
							},
							outputs: outputPkAErrorToBoth,
							logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
						},
						{
							description: "B sends a message out of order",
							input:       ectpStepIceBtoA.input,
							outputs:     outputPkBErrorToBoth,
							logs:        []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
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
								Pk:      &publicKey0,
								Msg:     "lol",
							},
							outputs: outputPkAErrorToBoth,
							logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
						},
						{
							description: "B sends bad input",
							input: model.RoutineInput{
								MsgType: model.RoutineMsgType_UsrMsg,
								Pk:      &publicKey1,
								Msg:     "lol",
							},
							outputs: outputPkBErrorToBoth,
							logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
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
							logs:        []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
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
							logs:        []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
						},
					},
				},
				{
					description: "A sends too many ice candidates",
					prefaceSteps: append(
						[]Step{
							ectpStepInitiateOnline,
							ectpStepAcceptAndOffer,
							ectpStepAnswer,
						},
						RepeatedSlice(ectpStepIceAToB, maxIceCandidates)..., // send `maxIceCandidates` number of the same message
					),

					cases: []Step{
						{
							description: "A sends one ICE candidate too many",
							input:       ectpStepIceAToB.input,
							outputs:     outputCustomErrorToBoth("You have sent too many ICE candidates", "Peer is sending too many ICE candidates"),
							logs:        []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
						},
					},
				},
				{
					description: "B sends too many ice candidates",
					prefaceSteps: append(
						[]Step{
							ectpStepInitiateOnline,
							ectpStepAcceptAndOffer,
							ectpStepAnswer,
						},
						RepeatedSlice(ectpStepIceBtoA, maxIceCandidates)..., // send `maxIceCandidates` number of the same message
					),

					cases: []Step{
						{
							description: "B sends one ICE candidate too many",
							input:       ectpStepIceBtoA.input,
							outputs:     outputCustomErrorToBoth("Peer is sending too many ICE candidates", "You have sent too many ICE candidates"),
							logs:        []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL)},
						},
					},
				},
			}

			for _, test := range tests {

				for j, testCase := range test.cases {

					t.Run(test.description+"-"+strconv.Itoa(j), func(t *testing.T) {

						clientA := &model.Client{IpAddr: ip0}
						clientA.SetPublicKey(&publicKey0)
						clientB := &model.Client{IpAddr: ip0}
						clientB.SetPublicKey(&publicKey1)
						hub := model.NewHub(testAllowedHostnames)
						hub.AddClient(publicKey0, clientA)
						hub.AddClient(publicKey1, clientB)
						var logOutput bytes.Buffer
						mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(establishConnectionToPeerLoggerAttrs...)
						ectp := newEstablishConnectionToPeer(clientA, hub, mockLogger)

						testRunner(t, ectp, &logOutput, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})
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
			"pattern": "` + publicKeyPattern + `"
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

func ectpLog(level string, kind string, msg ...string) ExpectedLog {
	var _msg = ""
	if len(msg) > 0 {
		_msg = msg[0]
	}
	return ExpectedLog{
		level: level,
		kind:  kind,
		client: &ClientLogAttributes{
			pk: string(publicKey0),
			ip: ip0,
		},
		transaction: &TransactionLogAttributes{
			routine: establishConnectionToPeerRoutineName,
			tsid:    tsid1,
		},
		msg: _msg,
	}
}

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
		Pk:      &publicKey0,
		Msg: `{
			"initiate": "sendConnectionRequest",
			"key": "` + (string)(publicKey1) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			verifyTimeouts: true,
			ro: model.RoutineOutput{
				Pk:              &publicKey1,
				Msgs:            []string{ectpSchemaInitiateToB},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
	logs: []ExpectedLog{
		ectpLog("INFO", ROUTINE_ADD_PK, string(publicKey1)),
	},
}

var ectpStepInitiateOffline = Step{
	description: "A sends a request, B is offline",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey0,
		Msg: `{
			"initiate": "sendConnectionRequest",
			"key": "` + (string)(publicKey1) + `"
		}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Msgs: []string{ectpSchemaOfflineToA},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{
		ectpLog("INFO", ROUTINE_ADD_PK_OFFLINE, string(publicKey1)),
		ectpLog("INFO", ROUTINE_SUCCEED),
	},
}

var ectpStepAcceptAndOffer = Step{
	description: "B sends an offer and server passes it to A",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey1,
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
				Pk:              &publicKey0,
				Msgs:            []string{ectpSchemaAcceptAndOfferToA(sdpOffer)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpExpectedTimeoutDuration,
			},
		},
	},
}

var ectpStepReject = Step{
	description: "Friend rejects connection request, server terminates both clients",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey1,
		Msg: `{
			"forward": {
				"type": "reject"
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
				Msgs: []string{ectpSchemaRejectToA},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{
		ectpLog("INFO", ROUTINE_SUCCEED, "connection request reject"),
	},
}

var ectpStepAnswer = Step{
	description: "A sends an answer and server passes it to B",
	input: model.RoutineInput{
		Pk:      &publicKey0,
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
				Pk:              &publicKey1,
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
		Pk:      &publicKey0,
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
				Pk:              &publicKey1,
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
		Pk:      &publicKey1,
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
				Pk:              &publicKey0,
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
		Pk:      &publicKey0,
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
				Pk:              &publicKey1,
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
		Pk:      &publicKey0,
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
				Pk:   &publicKey1,
				Msgs: []string{ectpSchemaIceCandidate(ICECandidateDone), schemaBareTerminate},
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Msgs: []string{schemaBareTerminate},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{
		ectpLog("INFO", ROUTINE_SUCCEED, "peers connect"),
	},
}

var ectpStepFinalIceB = Step{
	description: "B sends an empty ICE candidate to denote end of ice candidates, server passes it to A",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey1,
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
				Pk:              &publicKey0,
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
		Pk:      &publicKey1,
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
				Pk:   &publicKey0,
				Msgs: []string{ectpSchemaIceCandidate(ICECandidateDone), schemaBareTerminate},
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey1,
				Msgs: []string{schemaBareTerminate},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{
		ectpLog("INFO", ROUTINE_SUCCEED, "peers connect"),
	},
}

var stepPkADisconnect = Step{
	description: "A disconnects",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_ClientClose,
		Pk:      &publicKey0,
	},
	outputs: outputPkADisconnectedToB,
	logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "pk A close")},
}

var stepPkBDisconnect = Step{
	description: "B disconnects",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_ClientClose,
		Pk:      &publicKey1,
	},
	outputs: outputPkBDisconnectedToA,
	logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "pk B close")},
}

var stepPkACancel = Step{
	description: "A cancels",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey0,
		Msg:     `{"terminate":"cancel"}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey1,
				Msgs: []string{errorSchemaString("Peer cancelled the transaction")},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "pk A cancel")},
}
var stepPkBCancel = Step{
	description: "B cancels",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Pk:      &publicKey1,
		Msg:     `{"terminate":"cancel"}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey1,
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Msgs: []string{errorSchemaString("Peer cancelled the transaction")},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "pk B cancel")},
}

var stepPkATimeout = Step{
	description: "A times out",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_Timeout,
		Pk:      &publicKey0,
	},
	outputs: outputPkATimeoutToBoth,
	logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "pk A timeout")},
}

var stepPkBTimeout = Step{
	description: "B times out",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_Timeout,
		Pk:      &publicKey1,
	},
	outputs: outputPkBTimeoutToBoth,
	logs:    []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "pk B timeout")},
}

var outputPkAError = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey0,
			Msgs: []string{errorSchemaString()},
			Done: true,
		},
	},
}

var outputPkAErrorToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey0,
			Msgs: []string{errorSchemaString()},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey1,
			Msgs: []string{errorSchemaString("Peer sent a malformed message")},
			Done: true,
		},
	},
}

func outputCustomErrorToBoth(toA string, toB string) []ExpectedOutput {
	return []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey0,
				Msgs: []string{errorSchemaString(toA)},
				Done: true,
			},
		},
		{
			ro: model.RoutineOutput{
				Pk:   &publicKey1,
				Msgs: []string{errorSchemaString(toB)},
				Done: true,
			},
		},
	}
}

var outputPkATimeoutToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey0,
			Msgs: []string{errorSchemaString("Timeout")},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{

			Pk:   &publicKey1,
			Msgs: []string{errorSchemaString("Peer timed out")},
			Done: true,
		},
	},
}

var outputPkBErrorToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey1,
			Msgs: []string{errorSchemaString()},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{

			Pk:   &publicKey0,
			Msgs: []string{errorSchemaString("Peer sent a malformed message")},
			Done: true,
		},
	},
}

var outputPkBTimeoutToBoth = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey1,
			Msgs: []string{errorSchemaString("Timeout")},
			Done: true,
		},
	},
	{
		ro: model.RoutineOutput{

			Pk:   &publicKey0,
			Msgs: []string{errorSchemaString("Peer timed out")},
			Done: true,
		},
	},
}

var outputPkADisconnectedToB = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey1,
			Msgs: []string{errorSchemaString("Peer disconnected")},
			Done: true,
		},
	},
}

var outputPkBDisconnectedToA = []ExpectedOutput{
	{
		ro: model.RoutineOutput{
			Pk:   &publicKey0,
			Msgs: []string{errorSchemaString("Peer disconnected")},
			Done: true,
		},
	},
}
