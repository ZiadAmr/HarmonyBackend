package routines

import (
	"encoding/json"
	"harmony/backend/model"

	"github.com/xeipuuv/gojsonschema"
)

type FriendRejection struct {
	hub *model.Hub
	pkA *model.PublicKey
	pkB *model.PublicKey
}

func newFriendRejection(client *model.Client, hub *model.Hub) model.Routine {
	return &FriendRejection{hub: hub}
}

func (r *FriendRejection) Next(args model.RoutineInput) []model.RoutineOutput {
	// only 1 step, don't need to worry about state.
	r.pkA = args.Pk
	if r.pkA == nil {
		return frejError(nil, "You have not provided a public key")
	}

	// validate msg
	usrMsgLoader := gojsonschema.NewStringLoader(args.Msg)
	result, err := frejSchema.Validate(usrMsgLoader)
	if err != nil {
		return frejError(nil, err.Error())
	}
	if !result.Valid() {
		return frejError(nil, formatJSONError(result))
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
		return ectpError(nil, "You can't reject yourself")
	}

	_, peerOnline := r.hub.GetClient(*r.pkB)

	if peerOnline {
		return []model.RoutineOutput{
			{
				Pk:   r.pkA,
				Done: true,
				Msgs: []string{`{"peerStatus":"online","terminate":"done"}`},
			},
			{
				Pk:   r.pkB,
				Done: true,
				Msgs: []string{`{"initiate":"receiveFriendRejection","terminate":"done","key":"` + publicKeyToString(*r.pkA) + `"}`},
			},
		}
	} else {
		return []model.RoutineOutput{
			{
				Pk:   r.pkA,
				Done: true,
				Msgs: []string{`{"peerStatus":"offline","terminate":"done"}`},
			},
		}
	}
}

var frejSchema = func() *gojsonschema.Schema {
	schemaStr := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"initiate": {
				"const":"sendFriendRejection"
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

// wrapper for error routine output
func frejError(pk *model.PublicKey, msgs ...string) []model.RoutineOutput {
	return []model.RoutineOutput{
		{
			Pk:   pk,
			Done: true,
			Msgs: []string{MakeJSONError(msgs...)},
		},
	}
}
