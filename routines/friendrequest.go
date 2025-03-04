package routines

import (
	"encoding/json"
	"harmony/backend/model"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

type FRState int

const (
	fr_entry FRState = iota
	fr_reply
)

const frTimeOut = 10 * time.Second

type FriendRequest struct {
	pkA   *model.PublicKey
	pkB   *model.PublicKey
	hub   *model.Hub
	state FRState
}

func newFriendRequest(client *model.Client, hub *model.Hub) model.Routine {
	return &FriendRequest{
		hub:   hub,
		state: fr_entry,
	}
}

func (r *FriendRequest) Next(args model.RoutineInput) []model.RoutineOutput {

	switch args.MsgType {
	case model.RoutineMsgType_Timeout:
		ros := frError(nil, "Timeout")
		switch *args.Pk {
		case *r.pkA:
			ros = append(ros, frError(r.pkB, "Peer timed out")...)
		case *r.pkB:
			ros = append(ros, frError(r.pkA, "Peer timed out")...)
		}
		return ros
	case model.RoutineMsgType_ClientClose:
		// terminate the other person
		switch *args.Pk {
		case *r.pkA:
			return frError(r.pkB, "Peer disconnected")
		case *r.pkB:
			return frError(r.pkA, "Peer disconnected")
		default:
			panic("unknown pk")
		}
	case model.RoutineMsgType_UsrMsg:
		if isClientCancelMsg(args.Msg) {
			return r.cancel(args)
		}
		switch r.state {
		case fr_entry:
			return r.entry(args)
		case fr_reply:
			return r.reply(args)
		default:
			panic("unrecognized state")
		}
	default:
		panic("unrecognized message type")
	}

}

var frEntrySchema = func() *gojsonschema.Schema {
	schemaStr := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"initiate": {
				"const":"sendFriendRequest"
			},
			"key": {
				"type":"string",
				"pattern": "` + publicKeyPattern + `"
			}
		},
		"required": ["initiate", "key"],
		"additionalProperties": false
	}`
	schemaLoader := gojsonschema.NewStringLoader(schemaStr)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func (r *FriendRequest) entry(args model.RoutineInput) []model.RoutineOutput {

	// save pkA
	r.pkA = args.Pk
	if r.pkA == nil {
		return frError(nil, "You have not provided a public key")
	}

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := frEntrySchema.Validate(usrMsgLoader)
	if err != nil {
		return frError(nil, err.Error())
	}
	if !result.Valid() {
		return frError(nil, formatJSONError(result))
	}

	// parse msg
	usrMsg := struct {
		Initiate string `json:"initiate"`
		Key      string `json:"key"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)
	r.pkB, _ = parsePublicKey(usrMsg.Key)

	// check pkB is different from pkA
	if *(r.pkA) == *(r.pkB) {
		return ectpError(nil, "Sending a friend request to yourself is not allowed")
	}

	_, peerOnline := r.hub.GetClient(*r.pkB)

	if peerOnline {
		r.state = fr_reply
		return []model.RoutineOutput{
			{
				Pk:              r.pkB,
				TimeoutDuration: frTimeOut,
				TimeoutEnabled:  true,
				Msgs:            []string{`{"initiate":"receiveFriendRequest","key":"` + publicKeyToString(*r.pkA) + `"}`},
			},
		}
	} else {
		return []model.RoutineOutput{
			{
				Msgs: []string{`{"peerStatus":"offline","forwarded":null,"terminate":"done"}`},
				Done: true,
			},
		}
	}

}

var frReplySchema = func() *gojsonschema.Schema {
	schemaStr := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"forward": {
				"properties": {
					"type": {
						"enum": ["reject", "accept", "pending"]
					}
				},
				"additionalProperties": false,
				"required": ["type"]
			}
		},
		"required": ["forward"],
		"additionalProperties": false
	}`
	schemaLoader := gojsonschema.NewStringLoader(schemaStr)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func (r *FriendRequest) reply(args model.RoutineInput) []model.RoutineOutput {

	// check it's the correct pk
	if args.Pk == nil || *args.Pk == *r.pkA {
		return append(frError(nil, "Message send out of order"), frError(r.pkB, "Peer sent a malformed message")...)
	}

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := frReplySchema.Validate(usrMsgLoader)
	if err != nil {
		return append(frError(nil, err.Error()), frError(r.pkA, "Peer sent a malformed message")...)
	}
	if !result.Valid() {
		return append(frError(nil, formatJSONError(result)), frError(r.pkA, "Peer sent a malformed message")...)
	}

	// parse msg
	usrMsg := struct {
		Forward struct {
			Type string `json:"type"`
		} `json:"forward"`
	}{}
	json.Unmarshal([]byte(args.Msg), &usrMsg)

	return []model.RoutineOutput{
		{
			Pk:   r.pkA,
			Done: true,
			Msgs: []string{`{"peerStatus":"online","forwarded":{"type":"` + usrMsg.Forward.Type + `"},"terminate":"done"}`},
		},
		{
			Pk:   r.pkB,
			Done: true,
			Msgs: []string{`{"terminate":"done"}`},
		},
	}
}

func (r *FriendRequest) cancel(args model.RoutineInput) []model.RoutineOutput {
	if r.state == fr_entry {
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
			frError(peer, "Peer cancelled the transaction")[0],
		}
	}
}

// wrapper for error routine output
func frError(pk *model.PublicKey, msgs ...string) []model.RoutineOutput {
	return []model.RoutineOutput{
		{
			Pk:   pk,
			Done: true,
			Msgs: []string{MakeJSONError(msgs...)},
		},
	}
}
