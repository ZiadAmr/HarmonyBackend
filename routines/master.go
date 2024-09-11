package routines

import (
	"encoding/json"
	"fmt"
	"harmony/backend/model"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// helper function to convert json schema parse error to string
func formatJSONError(result *gojsonschema.Result) string {
	var errorStrings []string
	for _, error := range result.Errors() {
		errorStrings = append(errorStrings, error.Description())
	}
	return strings.Join(errorStrings, ", ")
}

const VERSION = "0.0"

// list of acceptable values of the `"initiate":` property
var routineNames = []string{"comeOnline", "establishConnectionToPeer"}

// schema to look for and validate the "initiate:" property
var initiateSchema = func() *gojsonschema.Schema {

	quotedRoutineNames := make([]string, len(routineNames))
	for i, val := range routineNames {
		quotedRoutineNames[i] = fmt.Sprintf(`"%s"`, val)
	}
	// string that looks like: "comeOnline","sendFriendRequest",...
	joinedQuotedRoutineNames := strings.Join(quotedRoutineNames, ",")

	stringSchema := fmt.Sprintf(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"initiate": {"enum": [%s]} 
		},
		"required": ["initiate"]
	}`, joinedQuotedRoutineNames)

	var schemaLoader = gojsonschema.NewStringLoader(stringSchema)
	var schema, _ = gojsonschema.NewSchema(schemaLoader)
	return schema
}()

// type for parsed json
type InitiateMessage struct {
	Initiate string
}

// Main router when a new transaction is started.
func MasterRoutine(client *model.Client, fromCl chan string, toCl chan string) {
	r := &RoutinesDefn{}
	masterRoutine(r, client, fromCl, toCl)
}

// abstracted routine functions for testing/dependency injection
type Routines interface {
	ComeOnline(client *model.Client, fromCl chan string, toCl chan string, errCl chan string)
	EstablishConnectionToPeer()
}

// version with mocks for testing purposes.
func masterRoutine(r Routines, client *model.Client, fromCl chan string, toCl chan string) {

	firstMessage := <-fromCl
	message := gojsonschema.NewStringLoader(firstMessage)

	// check that user message contains `"initiate":` property with a valid value
	result, err := initiateSchema.Validate(message)

	if err != nil {
		// client send malformed json
		return
	}
	if !result.Valid() {
		return
	}

	parsed := InitiateMessage{}
	err = json.Unmarshal([]byte(firstMessage), &parsed)

	if err != nil {
		panic(err.Error())
	}

	// do nothing with this currently
	errCl := make(chan string, 1)

	switch parsed.Initiate {
	case "comeOnline":
		r.ComeOnline(client, fromCl, toCl, errCl)
	case "establishConnectionToPeer":
		r.EstablishConnectionToPeer()
	default:
		panic("Unrecognized routine")
	}

}
