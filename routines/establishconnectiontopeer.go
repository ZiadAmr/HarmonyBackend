package routines

import (
	"encoding/json"
	"harmony/backend/model"
	"strconv"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

type ECTPState int

const ectpTimeoutDuration = 10 * time.Second

const (
	ectp_entry ECTPState = iota
	ectp_bAcceptOrReject
	ectp_aSdpAnswer
	ectp_iceCandidates
)

type EstablishConnectionToPeer struct {
	pkA                         *model.PublicKey
	pkB                         *model.PublicKey
	pkAHasSentEmptyICECandidate bool
	pkBHasSentEmptyICECandidate bool
	hub                         *model.Hub
	state                       ECTPState
}

func newEstablishConnectionToPeer(client *model.Client, hub *model.Hub) model.Routine {
	return &EstablishConnectionToPeer{
		hub:   hub,
		state: ectp_entry,
	}
}

func (r *EstablishConnectionToPeer) Next(args model.RoutineInput) []model.RoutineOutput {

	switch args.MsgType {
	case model.RoutineMsgType_Timeout:
		// note: assumption I am making here: if the pkA is set that means that pkA is online, same for pkB
		// these are never explicitly unset, however in the correct operation pkA and pkB's transaction sockets are closed at the same time
		// therefore it is not possible to receieve a timeout or a client close when only 1 of A and B has been terminated
		ros := ectpError(nil, "Timeout")

		if r.pkA != nil && r.pkB != nil {
			switch *args.Pk {
			case *r.pkA:
				ros = append(ros, ectpError(r.pkB, "Peer timed out")...)
			case *r.pkB:
				ros = append(ros, ectpError(r.pkA, "Peer timed out")...)
			}
		}
		return ros

	case model.RoutineMsgType_ClientClose:
		// terminate the other person
		if r.pkA != nil && r.pkB != nil {
			switch *args.Pk {
			case *r.pkA:
				return ectpError(r.pkB, "Peer disconnected")
			case *r.pkB:
				return ectpError(r.pkA, "Peer disconnected")
			}
		}
		return []model.RoutineOutput{}

	case model.RoutineMsgType_UsrMsg:

		if isClientCancelMsg(args.Msg) {
			return r.cancel(args)
		}
		switch r.state {
		case ectp_entry:
			return r.entry(args)
		case ectp_bAcceptOrReject:
			return r.bAcceptOrReject(args)
		case ectp_aSdpAnswer:
			return r.aSdpAnswer(args)
		case ectp_iceCandidates:
			return r.iceCandidates(args)
		default:
			panic("unrecognized state?")
		}
	default:
		panic("unrecognized message type")
	}

}

// client sends a {"terminate":"cancel"} message.
func (r *EstablishConnectionToPeer) cancel(args model.RoutineInput) []model.RoutineOutput {
	if r.state == ectp_entry {
		return []model.RoutineOutput{{
			Done: true,
		}}
	} else {
		var peer = r.pkA
		if *args.Pk == *r.pkA {
			peer = r.pkB
		}
		return []model.RoutineOutput{
			{
				Done: true,
			},
			ectpError(peer, "Peer cancelled the transaction")[0],
		}
	}
}

var entrySchema = func() *gojsonschema.Schema {
	errorSchemaString := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"initiate": {
				"const":"sendConnectionRequest"
			},
			"key": {
				"type":"string",
				"pattern": "^[0123456789abcdef]{` + strconv.Itoa(model.KEYLEN*2) + `}$"
			}
		},
		"required": ["initiate", "key"],
		"additionalProperties": false
	}`
	schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func (r *EstablishConnectionToPeer) entry(args model.RoutineInput) []model.RoutineOutput {

	// store public key of first peer
	if args.Pk == nil {
		return ectpError(nil, "You have not provided a public key")
	}
	r.pkA = args.Pk

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := entrySchema.Validate(usrMsgLoader)
	if err != nil {
		return ectpError(nil, err.Error())
	}
	if !result.Valid() {
		return ectpError(nil, formatJSONError(result))
	}

	// parse first message
	usrMsg := struct {
		Initiate string `json:"initiate"`
		Key      string `json:"key"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)
	r.pkB, _ = parsePublicKey(usrMsg.Key)

	_, peerOnline := r.hub.GetClient(*r.pkB)

	if peerOnline {
		r.state = ectp_bAcceptOrReject
		return []model.RoutineOutput{
			{
				Pk:              r.pkB,
				Msgs:            []string{`{"initiate":"receiveConnectionRequest","key":"` + publicKeyToString(*r.pkA) + `"}`},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpTimeoutDuration,
			},
		}
	} else {
		return []model.RoutineOutput{
			{
				Pk:   r.pkA,
				Msgs: []string{`{"peerStatus":"offline","forwarded":null,"terminate":"done"}`},
				Done: true,
			},
		}
	}

}

var bAcceptOrRejectSchema = func() *gojsonschema.Schema {
	errorSchemaString := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"forward": {
				"oneOf": [
					{
						"properties": {
							"type": {
								"const": "acceptAndOffer"
							},
							"payload": {
								"properties": {
									"type": {
										"const": "offer"
									},
									"sdp": {
										"type": "string"
									}
								},
								"required": ["type","sdp"],
								"additionalProperties": false
							}
						},
						"required": ["type", "payload"],
						"additionalProperties": false
					},
					{
						"properties": {
							"type": {
								"const": "reject"
							}
						},
						"required": ["type"],
						"additionalProperties": false
					}
				]
			}
		},
		"required": ["forward"],
		"additionalProperties": false
	}`
	schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func (r *EstablishConnectionToPeer) bAcceptOrReject(args model.RoutineInput) []model.RoutineOutput {

	// check response is from B
	if *args.Pk == *r.pkA {
		return append(ectpError(r.pkA, "Message sent out or order"), ectpError(r.pkB, "Peer sent a malformed message")...)
	}

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := bAcceptOrRejectSchema.Validate(usrMsgLoader)
	if err != nil {
		return append(ectpError(nil, err.Error()), ectpError(r.pkA, "Peer sent a malformed message")...)
	}
	if !result.Valid() {
		return append(ectpError(nil, formatJSONError(result)), ectpError(r.pkA, "Peer sent a malformed message")...)
	}

	usrMsg := struct {
		Forward struct {
			Type string `json:"type"`
		} `json:"forward"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)

	switch usrMsg.Forward.Type {
	case "reject":
		return []model.RoutineOutput{
			{
				Pk:   r.pkA,
				Msgs: []string{`{"peerStatus":"online","forwarded":{"type":"reject"},"terminate":"done"}`},
				Done: true,
			},
			{
				Pk: r.pkB,
				Msgs: []string{`{
					"terminate": "done"
				}`},
				Done: true,
			},
		}

	case "acceptAndOffer":

		// unmarshal the "payload" bit of the usrmsg
		usrMsgWithPayload := struct {
			Forward struct {
				Payload struct {
					Type string `json:"type"`
					Sdp  string `json:"sdp"`
				} `json:"payload"`
			} `json:"forward"`
		}{}
		json.Unmarshal([]byte(args.Msg), &usrMsgWithPayload)

		// create message to B
		// marshal it instead of creating the json string directly so that the SDPs get sanitized
		dataToB := struct {
			PeerStatus string `json:"peerStatus"`
			Forwarded  struct {
				Type    string `json:"type"`
				Payload struct {
					Type string `json:"type"`
					Sdp  string `json:"sdp"`
				} `json:"payload"`
			} `json:"forwarded"`
		}{}
		dataToB.PeerStatus = "online"
		dataToB.Forwarded.Type = "acceptAndOffer"
		dataToB.Forwarded.Payload.Type = "offer"
		dataToB.Forwarded.Payload.Sdp = usrMsgWithPayload.Forward.Payload.Sdp

		msgToA, _ := json.Marshal(dataToB)

		r.state = ectp_aSdpAnswer
		return []model.RoutineOutput{
			{
				Pk:              r.pkA,
				Msgs:            []string{string(msgToA)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpTimeoutDuration,
			},
		}

	default:
		panic(args.Msg)
	}

}

var aSdpAnswerSchema = func() *gojsonschema.Schema {
	errorSchemaString := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"forward": {
				"properties": {
					"type": {
						"const": "answer"
					},
					"payload": {
						"properties": {
							"type": {
								"const": "answer"
							},
							"sdp": {
								"type": "string"
							}
						},
						"required": ["type","sdp"],
						"additionalProperties": false
					}
				},
				"required": ["type","payload"],
				"additionalProperties": false
			}
		},
		"required": ["forward"],
		"additionalProperties": false
	}`
	schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func (r *EstablishConnectionToPeer) aSdpAnswer(args model.RoutineInput) []model.RoutineOutput {

	// reject any message from B
	if *args.Pk == *r.pkB {
		return append(ectpError(r.pkB, "Message sent out or order"), ectpError(r.pkA, "Peer sent a malformed message")...)
	}

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := aSdpAnswerSchema.Validate(usrMsgLoader)
	if err != nil {
		return append(ectpError(nil, err.Error()), ectpError(r.pkB, "Peer sent a malformed message")...)
	}
	if !result.Valid() {
		return append(ectpError(nil, formatJSONError(result)), ectpError(r.pkB, "Peer sent a malformed message")...)
	}

	// parse msg
	usrMsg := struct {
		Forward struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Sdp  string `json:"sdp"`
			} `json:"payload"`
		} `json:"forward"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)

	// remarshal it for B
	dataToB := struct {
		Forwarded struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Sdp  string `json:"sdp"`
			} `json:"payload"`
		} `json:"forwarded"`
	}{}
	dataToB.Forwarded.Type = "answer"
	dataToB.Forwarded.Payload.Type = "answer"
	dataToB.Forwarded.Payload.Sdp = usrMsg.Forward.Payload.Sdp
	msgToB, _ := json.Marshal(dataToB)

	r.state = ectp_iceCandidates
	return []model.RoutineOutput{
		{
			Pk:              r.pkB,
			Msgs:            []string{string(msgToB)},
			TimeoutEnabled:  true,
			TimeoutDuration: ectpTimeoutDuration,
		},
	}
}

var iceCandidatesSchema = func() *gojsonschema.Schema {
	errorSchemaString := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"forward": {
				"properties": {
					"type": {
						"const": "ICECandidate"
					},
					"payload": {
						"properties": {
							"candidate": {
								"type": "string"
							},
							"sdpMLineIndex": {
								"type": "integer"
							},
							"sdpMid": {
								"type": "string"
							},
							"usernameFragment": {
								"type": "string"
							}
						},
						"required": ["candidate","sdpMLineIndex","sdpMid","usernameFragment"],
						"additionalProperties": false
					}
				},
				"required": ["type","payload"],
				"additionalProperties": false
			}
		},
		"required": ["forward"],
		"additionalProperties": false
	}`
	schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func (r *EstablishConnectionToPeer) iceCandidates(args model.RoutineInput) []model.RoutineOutput {

	// set to true if both clients have finished sending ICE candidates.
	terminate := false

	// check who is sending the ice candidate
	// reject messages sent by a client who has already sent an empty ICE candidate (indicating that they had finished sending messages)
	var toPk *model.PublicKey
	if *args.Pk == *r.pkA {
		toPk = r.pkB
		if r.pkAHasSentEmptyICECandidate {
			return append(ectpError(nil, "Another ICE candidate sent after final ICE candidate"), ectpError(toPk, "Peer sent a malformed message")...)
		}
	} else if *args.Pk == *r.pkB {
		toPk = r.pkA
		if r.pkBHasSentEmptyICECandidate {
			return append(ectpError(nil, "Another ICE candidate sent after final ICE candidate"), ectpError(toPk, "Peer sent a malformed message")...)
		}
	} else {
		panic("received ice candidate from unknown client")
	}

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := iceCandidatesSchema.Validate(usrMsgLoader)
	if err != nil {
		return append(ectpError(nil, err.Error()), ectpError(toPk, "Peer sent a malformed message")...)
	}
	if !result.Valid() {
		return append(ectpError(nil, formatJSONError(result)), ectpError(toPk, "Peer sent a malformed message")...)
	}

	// parse msg
	usrMsg := struct {
		Forward struct {
			Type    string `json:"type"`
			Payload struct {
				Candidate        string `json:"candidate"`
				SdpMLineIndex    int    `json:"sdpMLineIndex"`
				SdpMid           string `json:"sdpMid"`
				UsernameFragment string `json:"usernameFragment"`
			} `json:"payload"`
		} `json:"forward"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)

	// check for end of ice candidates (empty candidate field)
	if usrMsg.Forward.Payload.Candidate == "" {
		switch *args.Pk {
		case *r.pkA:
			r.pkAHasSentEmptyICECandidate = true
			if r.pkBHasSentEmptyICECandidate {
				terminate = true
			}
		case *r.pkB:
			r.pkBHasSentEmptyICECandidate = true
			if r.pkAHasSentEmptyICECandidate {
				terminate = true
			}
		}
	}

	// remarshal
	forwardedData := struct {
		Forwarded struct {
			Type    string `json:"type"`
			Payload struct {
				Candidate        string `json:"candidate"`
				SdpMLineIndex    int    `json:"sdpMLineIndex"`
				SdpMid           string `json:"sdpMid"`
				UsernameFragment string `json:"usernameFragment"`
			} `json:"payload"`
		} `json:"forwarded"`
	}{}
	forwardedData.Forwarded.Type = usrMsg.Forward.Type
	forwardedData.Forwarded.Payload.Candidate = usrMsg.Forward.Payload.Candidate
	forwardedData.Forwarded.Payload.SdpMLineIndex = usrMsg.Forward.Payload.SdpMLineIndex
	forwardedData.Forwarded.Payload.SdpMid = usrMsg.Forward.Payload.SdpMid
	forwardedData.Forwarded.Payload.UsernameFragment = usrMsg.Forward.Payload.UsernameFragment

	forwardedStr, _ := json.Marshal(forwardedData)

	if terminate {
		return []model.RoutineOutput{
			{
				Pk:   toPk,
				Msgs: []string{string(forwardedStr), terminateDoneJSONMsg()},
				Done: true,
			},
			{
				Pk:   nil, // sender
				Msgs: []string{terminateDoneJSONMsg()},
				Done: true,
			},
		}
	} else {
		return []model.RoutineOutput{
			{
				Pk:              toPk,
				Msgs:            []string{string(forwardedStr)},
				TimeoutEnabled:  true,
				TimeoutDuration: ectpTimeoutDuration,
			},
		}
	}
}

// wrapper for error routine output
func ectpError(pk *model.PublicKey, msgs ...string) []model.RoutineOutput {
	return []model.RoutineOutput{
		{
			Pk:   pk,
			Done: true,
			Msgs: []string{MakeJSONError(msgs...)},
		},
	}
}
