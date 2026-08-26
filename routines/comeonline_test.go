package routines

import (
	"bytes"
	"encoding/json"
	"harmony/backend/model"
	"io"
	"log/slog"
	"strconv"
	"strings"
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
var comeOnlineChallengeResponseSchema = func(message ...string) string {
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
    "challenge": ` + frag + `
  },
  "required": ["challenge"],
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

const comeOnlineRoutineName = "comeOnline"

var comeOnlineLoggerAttrs = toAnySlice("ip", ip0, "pk", "nil", "routine", comeOnlineRoutineName, "tsid", tsid1)

func TestComeOnline(t *testing.T) {

	t.Run("runs correctly on valid inputs", func(t *testing.T) {

		// define tests
		tests := []struct {
			key              model.PublicKey
			challenge        string
			currentTime      string
			allowedHostnames []string
			steps            []Step
		}{
			{
				key:              publicKey0,
				challenge:        testMessage,
				currentTime:      testTime,
				allowedHostnames: []string{"harmonytestserver.org"},
				steps: []Step{
					coStepInitiate,
					coStepValidPk(publicKey0, testMessage),
					coStepValidSignature(createSignatureMessage(testMessage, "harmonytestserver.org", "2023-09-24T15:30:00Z", "dQNw2g5cNppO5a20139Z/xjiT3NPY3AQWWQhP6j+zMdMNLxiWNCuISJhiPo9A01U++dQz5HDLuL8twxiUQRrBw==")),
				},
			},
			{
				key:              publicKey0,
				challenge:        testMessage,
				currentTime:      testTime,
				allowedHostnames: []string{"0.0.0.0"}, // allow any hostname
				steps: []Step{
					coStepInitiate,
					coStepValidPk(publicKey0, testMessage),
					coStepValidSignature(createSignatureMessage(testMessage, "anywhere.com", "2023-09-24T15:30:00Z", "b9iX1k8wRj/Lk50KyApsM2Egmx4qxiAktvRN+hLYjnx/hdp4ztSlDib9Q/yPBwtvPGpkdyVljHPsdvqzKuc0AA==")),
				},
			},
			{
				key:              publicKey0,
				challenge:        testMessage,
				currentTime:      testTime,
				allowedHostnames: []string{"harmonytestserver.org"},
				steps: []Step{
					coStepInitiate,
					coStepValidPk(publicKey0, testMessage),
					coStepValidSignature(createSignatureMessage(testMessage, "harmonytestserver.org", "2023-09-24T15:30:01Z", "yOxXxrbGwJ3ahUbHRvvegxddmkKxiDpSnNu31oriLYvNuwhGpCeKhgQ/uE98pambs4QenGqIhrgUGyFrliagDg==")),
				},
			},
		}

		for i, tt := range tests {

			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				mockClient := &model.Client{IpAddr: ip0}
				mockHub := model.NewHub(tt.allowedHostnames)
				mockRndMsgGen := fixedMessageGenerator{tt.challenge}
				mockCurrentTimeGen := fixedTimeGen{tt.currentTime}
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)

				co := newComeOnlineDependencyInj(mockClient, mockHub, mockLogger, mockRndMsgGen, mockCurrentTimeGen)

				testRunner(t, co, &logOutput, tt.steps)

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

					mockClient := &model.Client{IpAddr: ip0}
					mockHub := model.NewHub(testAllowedHostnames)
					var logOutput bytes.Buffer
					mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)
					co := newComeOnline(mockClient, mockHub, mockLogger)

					testRunner(t, co, &logOutput, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})
				})

			}
		}
	})

	t.Run("cancels transaction if public key is already signed in on another connection", func(t *testing.T) {

		steps := []Step{
			coStepInitiate,
			coStepPkAlreadySignedIn,
		}

		// mock hub with the client already signed in
		hub := model.NewHub(testAllowedHostnames)

		client0 := &model.Client{IpAddr: ip1}
		key := publicKey0
		client0.SetPublicKey(&key)
		hub.AddClient(key, client0)

		// client that tries to use a public key that is already signed in
		client1 := &model.Client{IpAddr: ip0}
		var logOutput bytes.Buffer
		mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)

		co := newComeOnline(client1, hub, mockLogger)

		testRunner(t, co, &logOutput, steps)

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
				logs: []ExpectedLog{coLog("INFO", ROUTINE_FAIL, "Concurrent comeOnline")},
			},
		}

		for i, test := range co0Tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				client := &model.Client{IpAddr: ip0}
				hub := model.NewHub(testAllowedHostnames)
				mockRndMsgGen := fixedMessageGenerator{testMessage}
				mockCurrentTimeGen := fixedTimeGen{testTime}
				mockLogger0 := slog.New(slog.NewJSONHandler(io.Discard, nil)).With(comeOnlineLoggerAttrs...)
				co0 := newComeOnlineDependencyInj(client, hub, mockRndMsgGen, mockCurrentTimeGen, mockLogger0)

				// manually run the first test - after this point it is not complete
				for _, step := range test {
					co0.Next(step.input)
				}

				// start another comeOnline
				var logOutput1 bytes.Buffer
				mockLogger1 := slog.New(slog.NewJSONHandler(&logOutput1, nil)).With(comeOnlineLoggerAttrs...)
				co1 := newComeOnlineDependencyInj(client, hub, mockRndMsgGen, mockCurrentTimeGen, mockLogger1)
				testRunner(t, co1, &logOutput1, co1Test) // expect it to fail
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
			coStepValidSignature(createSignatureMessage(testMessage, testAllowedHostnames[0], testTime, "dQNw2g5cNppO5a20139Z/xjiT3NPY3AQWWQhP6j+zMdMNLxiWNCuISJhiPo9A01U++dQz5HDLuL8twxiUQRrBw==")),
		}

		for i, test := range co0Tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				client := &model.Client{IpAddr: ip0}
				hub := model.NewHub(testAllowedHostnames)
				mockRndMsgGen := fixedMessageGenerator{testMessage}
				mockCurrentTimeGen := fixedTimeGen{testTime}

				mockLogger0 := slog.New(slog.NewJSONHandler(io.Discard, nil)).With(comeOnlineLoggerAttrs...)
				co0 := newComeOnlineDependencyInj(client, hub, mockLogger0, mockRndMsgGen, mockCurrentTimeGen)

				// manually run the first test - it has completed at this point.
				for _, step := range test {
					co0.Next(step.input)
				}

				// start another comeOnline
				var logOutput1 bytes.Buffer
				mockLogger1 := slog.New(slog.NewJSONHandler(&logOutput1, nil)).With(comeOnlineLoggerAttrs...)
				co1 := newComeOnlineDependencyInj(client, hub, mockLogger1, mockRndMsgGen, mockCurrentTimeGen)
				testRunner(t, co1, &logOutput1, co1Test) // expect it not to fail
			})
		}
	})

	t.Run("Rejects immediately if public key is already set", func(t *testing.T) {

		pk := publicKey0

		steps := []Step{
			coStepInitiatePkAlreadySet,
		}

		mockClient := &model.Client{IpAddr: ip0}
		mockClient.SetPublicKey(&pk)
		mockHub := model.NewHub(testAllowedHostnames)
		var logOutput bytes.Buffer
		mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)
		co := newComeOnline(mockClient, mockHub, mockLogger)

		testRunner(t, co, &logOutput, steps)

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
					logs: []ExpectedLog{
						coLog("INFO", ROUTINE_INIT, comeOnlineRoutineName),
						coLog("INFO", ROUTINE_FAIL, "pk A cancel"),
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
				mockClient := &model.Client{IpAddr: ip0}
				mockHub := model.NewHub(testAllowedHostnames)
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)
				co := newComeOnline(mockClient, mockHub, mockLogger)
				testRunner(t, co, &logOutput, tt)
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
				mockClient := &model.Client{IpAddr: ip0}
				mockHub := model.NewHub(testAllowedHostnames)
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)
				co := newComeOnline(mockClient, mockHub, mockLogger)
				testRunner(t, co, &logOutput, tt)
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
				mockClient := &model.Client{IpAddr: ip0}
				mockHub := model.NewHub(testAllowedHostnames)
				var logOutput bytes.Buffer
				mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)
				co := newComeOnline(mockClient, mockHub, mockLogger)
				testRunner(t, co, &logOutput, tt)
			})
		}

	})

	t.Run("Rejects incorrect/invalid signatures", func(t *testing.T) {

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
					coStepInvalidSignature(`{}`),
					coStepInvalidSignature(`}`),
					coStepInvalidSignature(`{"signature":"0NK9zFHF7UeyQpWPMC9BsrD+wJiEEN1NbywNdHS1URx+FrQT5yCri66CSIh41umXTxEYiXS+LujfbJJW+wRbDQ=="` /*No payload*/),
					coStepInvalidSignature(`{"payload": {}, "signature":"0NK9zFHF7UeyQpWPMC9BsrD+wJiEEN1NbywNdHS1URx+FrQT5yCri66CSIh41umXTxEYiXS+LujfbJJW+wRbDQ=="` /*Missing fields in payload*/),
					coStepInvalidSignature(createSignatureMessage(testMessage, testAllowedHostnames[0], testTime, "0NK9zFHF7UeyQpWPMC9BsrD+wJiEEN1NbywNdHS1URx+FrQT5yCri66CSIh41umXTxEYiXS+LujfbJJW+wRbDQ==") /*Signature invalid*/),
					coStepInvalidSignature(createSignatureMessage("OOPS", testAllowedHostnames[0], testTime, "N8XFYNhpgSd2T4NCdNpy1lP9akdjfx4XdlHRLpx8ewP8SmXCWdgwpRo882y+j3BRFfdy7pAK2mgjmPNt+fDeBQ==") /*incorrect challenge signed*/),
					coStepInvalidSignature(createSignatureMessage(testMessage, "google.com", testTime, "4Od8mhJXkrbFpa2y8gd942ZQjoRwtUtDSB2XkZ3+tufsoD0/iqjp4xvhdN2O1X/rZ1swilLOkQHT5KQmhrXLAQ==") /*Wrong hostname*/),
					coStepInvalidSignature(createSignatureMessage(testMessage, testAllowedHostnames[0], "5 o'clock", "g/sC1A6Y3qbmhPb96riBbgevNSZ667uq7MJpvocSqkVT63w2HLQEPFm12OUx7l5FSyZxxx0oUJtzp3pG+vLmAg==") /*Time not in rfc3339 format*/),
					coStepInvalidSignature(createSignatureMessage(testMessage, testAllowedHostnames[0], "2023-09-24T15:30:03Z", "ehahcbrkWdQvGt27/cHYmn023rCMNifzwD4IKRExjj71OF6uh02hS12eQcCgRRJU5sdw1gQE7nuTESsonfQACA==") /*Time out by more than 2 seconds*/),
					coStepInvalidSignature(strings.ReplaceAll(createSignatureMessage(testMessage, testAllowedHostnames[0], testTime, "hPCPlryKpf39jecSRsSjOZj9BIBkVIDqfha92JOTmtoKIn3uERvaUD/lYwQg6NjKJWcZLl1gUG0SDd3uyvvdCQ=="), "comeOnline", "hacking") /*Incorrect purpose: changed from "comeOnline" to "hacking"*/),
				},
			},
		}

		for _, test := range tests {

			for _, testCase := range test.cases {

				t.Run(test.description+"-"+testCase.input.Msg, func(t *testing.T) {

					mockClient := &model.Client{}
					mockHub := model.NewHub(testAllowedHostnames)
					mockRndMsgGen := fixedMessageGenerator{test.msgToSign}
					mockCurrentTimeGen := fixedTimeGen{testTime}
					var logOutput bytes.Buffer
					mockLogger := slog.New(slog.NewJSONHandler(&logOutput, nil)).With(comeOnlineLoggerAttrs...)
					co := newComeOnlineDependencyInj(mockClient, mockHub, mockLogger, mockRndMsgGen, mockCurrentTimeGen)

					testRunner(t, co, &logOutput, append(test.prefaceSteps, testCase), testRunnerConfig{errorsOnLastStepOnly: true})

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

func coLog(level string, kind string, msg ...string) ExpectedLog {
	var _msg = ""
	if len(msg) > 0 {
		_msg = msg[0]
	}
	return ExpectedLog{
		level: level,
		kind:  kind,
		client: &ClientLogAttributes{
			pk: "nil",
			ip: ip0,
		},
		transaction: &TransactionLogAttributes{
			routine: comeOnlineRoutineName,
			tsid:    tsid1,
		},
		msg: _msg,
	}
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
	logs: []ExpectedLog{coLog("INFO", ROUTINE_INIT, comeOnlineRoutineName)},
}

var coStepInitiatePkAlreadySet = Step{
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
	logs: []ExpectedLog{
		coLog("INFO", ROUTINE_INIT, comeOnlineRoutineName),
		coLog("INFO", ROUTINE_FAIL, "pk already set"),
	},
}

var coStepPkAlreadySignedIn = Step{
	description: "User provides public key, but a different client is already signed in with this key",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_UsrMsg,
		Msg:     `{"publicKey": "` + (string)(publicKey0) + `"}`,
	},
	outputs: []ExpectedOutput{
		{
			ro: model.RoutineOutput{
				Msgs: []string{errorSchemaString()},
				Done: true,
			},
		},
	},
	logs: []ExpectedLog{coLog("INFO", ROUTINE_FAIL)},
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
					Msgs: []string{comeOnlineChallengeResponseSchema(msgToSign...)},
				},
			},
		},
	}
}

var createSignatureMessage = func(challenge string, hostname string, currentTime string, signature string) string {
	input := struct {
		Payload struct {
			Challenge   string `json:"challenge"`
			Hostname    string `json:"hostname"`
			Purpose     string `json:"purpose"`
			CurrentTime string `json:"currentTime"`
		} `json:"payload"`
		Signature string `json:"signature"`
	}{}
	input.Payload.Challenge = challenge
	input.Payload.Hostname = hostname
	input.Payload.Purpose = "comeOnline"
	input.Payload.CurrentTime = currentTime
	input.Signature = signature

	inputMarshal, _ := json.Marshal(input)
	return string(inputMarshal)
}

var coStepValidSignature = func(signatureMessage string) Step {

	return Step{
		description: "User replies with the correct signature for the message, and server welcomes the user.",
		input: model.RoutineInput{
			MsgType: model.RoutineMsgType_UsrMsg,
			Msg:     signatureMessage,
		},
		outputs: []ExpectedOutput{
			{
				ro: model.RoutineOutput{
					Msgs: []string{comeOnlineWelcomeResponseSchema},
					Done: true,
				},
			},
		},
		logs: []ExpectedLog{coLog("INFO", ROUTINE_SUCCEED)},
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
	logs: []ExpectedLog{coLog("INFO", ROUTINE_FAIL, "pk A cancel")},
}

var coStepClientClose = Step{
	description: "Client close",
	input: model.RoutineInput{
		MsgType: model.RoutineMsgType_ClientClose,
	},
	// expect no output
	logs: []ExpectedLog{coLog("INFO", ROUTINE_FAIL, "pk A close")},
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
	logs: []ExpectedLog{coLog("INFO", ROUTINE_FAIL, "pk A timeout")},
}

var coStepInvalidSignature = func(signatureMessage string, errorMessage ...string) Step {
	return Step{
		description: "Client sends an invalid payload/signature message, server cancels the transaction and replies with an error message.",
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
		logs: []ExpectedLog{coLog("INFO", ROUTINE_FAIL, "Invalid signature")},
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
		logs: []ExpectedLog{ectpLog("INFO", ROUTINE_FAIL, "Bad public key")},
	}
}

const testMessage = "This is a test message used to verify the public key. Usually, it would consist of random characters. It is sent to the user, who hashes and signs it with their private key. The signature is sent back to this server, which verifies the signature against the public key."
const testNonce = "This is a test nonce. It is at most 100 characters generated by the client and appended to the msg."

var testAllowedHostnames = []string{"harmonytestserver.org"}

const testTime = "2023-09-24T15:30:00Z"

// testMessage signed with publicKey0
const testPk0Signature = "jIX/9ZHy6UuGZzywconx5rSV77yGugYg2M40ROilWS/zo3qnlau2Zn2p045ZYvKDH98LrMm8vJOmdmWBCkY0Bg=="

// testMessage++testNonce signed with publicKey0
const testPk0NonceSignature = "0NK9zFHF7UeyQpWPMC9BsrD+wJiEEN1NbywNdHS1URL+FrQT5yCri66CSIh41umXTxEYiXS+LujfbJJW+wRbDQ=="

// non-random message generator for mocking.
type fixedMessageGenerator struct {
	msg string
}

func (g fixedMessageGenerator) GetMessage() (string, error) {
	return g.msg, nil
}

// non-random current time for mocking
type fixedTimeGen struct {
	time string
}

func (g fixedTimeGen) GetCurrentTime() string {
	return g.time
}

type allowedHostnamesGen struct {
	AllowedHostnames []string
}

func (g allowedHostnamesGen) GetAllowedHostnames() []string {
	return g.AllowedHostnames
}
