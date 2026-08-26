package routines

import (
	"bytes"
	"harmony/backend/model"
	"log/slog"
	"testing"
)

const friendRejectionRoutineName = "friendRejection"

var friendRejectionLoggerAttrs = toAnySlice("ip", ip0, "pk", string(publicKey1), "routine", friendRejectionRoutineName, "tsid", tsid1)

func TestFriendRejection(t *testing.T) {

	t.Run("Valid inputs", func(t *testing.T) {

		t.Run("Peer is online", func(t *testing.T) {
			test := []Step{
				frejStepOnline,
			}

			clientA := &model.Client{IpAddr: ip0}
			clientA.SetPublicKey(&publicKey0)
			clientB := &model.Client{IpAddr: ip1}
			clientB.SetPublicKey(&publicKey1)
			hub := model.NewHub(testAllowedHostnames)
			hub.AddClient(*clientA.GetPublicKey(), clientA)
			hub.AddClient(*clientB.GetPublicKey(), clientB)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(friendRejectionLoggerAttrs...)
			fr := newFriendRejection(clientA, hub, mockLogger)

			testRunner(t, fr, &logOutput, test)

		})

		t.Run("Peer is offline", func(t *testing.T) {

			test := []Step{
				frejStepOffline,
			}

			clientA := &model.Client{IpAddr: ip0}
			clientA.SetPublicKey(&publicKey0)
			hub := model.NewHub(testAllowedHostnames)
			hub.AddClient(*clientA.GetPublicKey(), clientA)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(friendRejectionLoggerAttrs...)
			fr := newFriendRejection(clientA, hub, mockLogger)

			testRunner(t, fr, &logOutput, test)
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
					logs: []ExpectedLog{
						{
							level: "INFO",
							kind:  ROUTINE_FAIL,
							client: &ClientLogAttributes{
								ip: ip0,
								pk: "nil",
							},
							transaction: &TransactionLogAttributes{
								routine: friendRejectionRoutineName,
								tsid:    tsid1,
							},
						},
					},
				},
			}

			clientA := &model.Client{IpAddr: ip0}
			clientB := &model.Client{IpAddr: ip1}
			clientB.SetPublicKey(&publicKey1)
			hub := model.NewHub(testAllowedHostnames)
			hub.AddClient(*clientB.GetPublicKey(), clientB)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(friendRejectionLoggerAttrs...).With("pk", "nil")

			fr := newFriendRejection(clientA, hub, mockLogger)

			testRunner(t, fr, &logOutput, test)
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
					logs: []ExpectedLog{
						frejLog("INFO", ROUTINE_FAIL, "Send to self"),
					},
				},
			}

			clientA := &model.Client{IpAddr: ip0}
			clientA.SetPublicKey(&publicKey0)
			hub := model.NewHub(testAllowedHostnames)
			hub.AddClient(*clientA.GetPublicKey(), clientA)
			var logOutput bytes.Buffer
			mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(friendRejectionLoggerAttrs...)

			fr := newFriendRejection(clientA, hub, mockLogger)

			testRunner(t, fr, &logOutput, test)
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
				logs:    []ExpectedLog{frejLog("INFO", ROUTINE_FAIL)},
			},

			{
				description: "Key in wrong format",
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Pk:      &publicKey0,
					Msg:     `{"initiate": "sendFriendRejection", "key":"4"}`,
				},
				outputs: outputPkAError,
				logs:    []ExpectedLog{frejLog("INFO", ROUTINE_FAIL)},
			},

			{
				description: "Invalid JSON",
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Pk:      &publicKey0,
					Msg:     `)`,
				},
				outputs: outputPkAError,
				logs:    []ExpectedLog{frejLog("INFO", ROUTINE_FAIL)},
			},

			{
				description: "Extra properties",
				input: model.RoutineInput{
					MsgType: model.RoutineMsgType_UsrMsg,
					Pk:      &publicKey0,
					Msg:     `{"initiate": "sendFriendRejection", "key":"` + (string)(publicKey1) + `", "extraProperty!":{}}`,
				},
				outputs: outputPkAError,
				logs:    []ExpectedLog{frejLog("INFO", ROUTINE_FAIL)},
			},
		}

		for _, test := range tests {
			t.Run(test.description, func(t *testing.T) {
				clientA := &model.Client{IpAddr: ip0}
				clientA.SetPublicKey(&publicKey0)
				clientB := &model.Client{IpAddr: ip1}
				clientB.SetPublicKey(&publicKey1)
				hub := model.NewHub(testAllowedHostnames)
				hub.AddClient(*clientA.GetPublicKey(), clientA)
				hub.AddClient(*clientB.GetPublicKey(), clientB)
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(friendRejectionLoggerAttrs...)

				fr := newFriendRejection(clientA, hub, mockLogger)

				testRunner(t, fr, &logOutput, []Step{test})
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
	logs: []ExpectedLog{
		frejLog("INFO", ROUTINE_ADD_PK, string(publicKey1)),
		frejLog("INFO", ROUTINE_SUCCEED, "Friend rejection delivered"),
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
	logs: []ExpectedLog{
		frejLog("INFO", ROUTINE_ADD_PK_OFFLINE, string(publicKey1)),
		frejLog("INFO", ROUTINE_SUCCEED, "Friend rejection not delivered - peer is offline"),
	},
}

func frejLog(level string, kind string, msg ...string) ExpectedLog {
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
			routine: friendRejectionRoutineName,
			tsid:    tsid1,
		},
		msg: _msg,
	}
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
