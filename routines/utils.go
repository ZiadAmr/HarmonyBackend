package routines

import (
	"encoding/json"
	"harmony/backend/model"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

/*
Make error in format `{"terminate":"cancel", error: "..."}`

If no argument is supplied the `"error":"..."` part is omitted.
*/
func MakeJSONError(msg ...string) string {
	// if no arg just return the standard message
	if len(msg) == 0 {
		return `{"terminate":"cancel"}`
	}

	// create json using the first argument to this fn
	type JsonError struct {
		Terminate string `json:"terminate"`
		Error     string `json:"error"`
	}
	b, _ := json.Marshal(JsonError{Terminate: "cancel", Error: msg[0]})
	return string(b)
}

func terminateDoneJSONMsg() string {
	return `{"terminate":"done"}`
}

// error messages to send to the client should look like this.
var clientCancelSchema = func() *gojsonschema.Schema {
	schemaLoader := gojsonschema.NewStringLoader(`
	{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"terminate": {
				"const":"cancel"
			},
            "error": {
             	"type": "string" 
            }
		},
		"required": ["terminate"]
	}
	`)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

// {"terminate":"cancel"}
func isClientCancelMsg(msg string) bool {
	msgLoader := gojsonschema.NewStringLoader(msg)
	result, err := clientCancelSchema.Validate(msgLoader)
	return err == nil && result.Valid()
}

// helper function to convert json schema parse error to string
func formatJSONError(result *gojsonschema.Result) string {
	var errorStrings []string
	for _, error := range result.Errors() {
		errorStrings = append(errorStrings, error.Description())
	}
	return strings.Join(errorStrings, ", ")
}

var routineContructorImplementations = RoutineConstructors{
	NewComeOnline:                newComeOnline,
	NewEstablishConnectionToPeer: newEstablishConnectionToPeer,
	NewFriendRequest:             newFriendRequest,
	NewFriendRejection:           newFriendRejection,
}

func parsePublicKey(pkstr string) (*model.PublicKey, error) {
	// if len(pkstr) != publicKeyBase32Len {
	// 	return nil, errors.New("key incorrect length")
	// }
	// pk, err := base32.StdEncoding.DecodeString(pkstr)
	// if err != nil {
	// 	return nil, err
	// }

	// TODO check that it is a valid public key
	return (*model.PublicKey)(&pkstr), nil
}

func publicKeyToString(pk model.PublicKey) string {
	return (string)(pk)
}

// const publicKeyBase32Len = 472

// from https://stackoverflow.com/questions/475074/regex-to-parse-or-validate-base64-data
const publicKeyPattern = "^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
