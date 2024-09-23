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
func MasterRoutine(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string) {
	r := &RoutinesDefn{}
	masterRoutine(r, client, hub, fromCl, toCl)
}

// version with mocks for testing purposes.
func masterRoutine(r Routines, client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string) {

	firstMsg := <-fromCl
	message := gojsonschema.NewStringLoader(firstMsg)

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
	err = json.Unmarshal([]byte(firstMsg), &parsed)

	if err != nil {
		panic(err.Error())
	}

	// don't do anything with this currently
	errCl := make(chan string, 1)
	defer close(errCl)

	// we need to send all incoming messages (on fromCl) to the routine.
	// however, we've already popped the first message.
	// we need to make a new channel so we can send the first message and forward the proceeding messages
	fromClForward := make(chan string)
	defer close(fromClForward)

	// call correct routine
	done := make(chan struct{})
	defer close(done)
	go func() {
		switch parsed.Initiate {
		case "comeOnline":
			r.ComeOnline(client, hub, fromClForward, toCl, errCl)
		case "establishConnectionToPeer":
			r.EstablishConnectionToPeer(client, hub, fromClForward, toCl, errCl)
		default:
			panic("Unrecognized routine")
		}
		done <- struct{}{}
	}()

	// forward the first message and subsequent incoming messages to the routine until `done` received
	// horrible code, sorry about that.
	// the problem is that both reading from `fromCl` and writing to `fromClForward` could be blocking
	// and we also need to keep an eye on `done` at all times
	// not currently sure how to make it any better.
	forwardMessage := func(msg string) bool {
		select {
		case <-done:
			return false // message was not sent
		case fromClForward <- msg:
			return true // message was sent
		}
	}
	if !forwardMessage(firstMsg) {
		return
	}
	for {
		select {
		case <-done:
			return
		case msg := <-fromCl:
			if !forwardMessage(msg) {
				return
			}
		}
	}

}
