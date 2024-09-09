package routines

import (
	"fmt"
	"harmony/backend/model"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// list of acceptable values of the `"initiate":` property
var routineNames = []string{"comeOnline"}

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

// abstracted routine functions for testing/dependency injection
type Routines interface {
	ComeOnline()
}

func MasterRoutine(r Routines, client *model.Client, fromCl chan string, toCl chan string) {

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

	// would check the actual value of initiate here.
	r.ComeOnline()
}
