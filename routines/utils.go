package routines

import (
	"encoding/json"

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
		"required": ["terminate"],
		"additionalProperties": false
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
