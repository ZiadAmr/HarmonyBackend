package routines

import (
	"encoding/json"
	"errors"
	"fmt"
	"harmony/backend/model"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

const VERSION = "0.0"

type MasterRoutine struct {
	isSubRoutineSet bool
	subRoutine      model.Routine
	rc              RoutineConstructors
	client          *model.Client
	hub             *model.Hub
}

func NewMasterRoutine(client *model.Client, hub *model.Hub) model.Routine {
	return newMasterRoutineDependencyInj(routineContructorImplementations, client, hub)
}

func newMasterRoutineDependencyInj(rc RoutineConstructors, client *model.Client, hub *model.Hub) model.Routine {
	return &MasterRoutine{
		rc:     rc,
		client: client,
		hub:    hub,
	}
}

func (r *MasterRoutine) Next(args model.RoutineInput) []model.RoutineOutput {

	if !r.isSubRoutineSet {
		err := r.setSubRoutineFromInitialMsg(args.Msg)
		if err != nil {
			return []model.RoutineOutput{model.MakeRoutineOutput(true, MakeJSONError(err.Error()))}
		}
		r.isSubRoutineSet = true
	}

	return r.subRoutine.Next(args)
}

// list of acceptable values of the `"initiate":` property
var routineNames = []string{"comeOnline", "sendConnectionRequest", "sendFriendRequest"}

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

func (r *MasterRoutine) setSubRoutineFromInitialMsg(msg string) error {

	message := gojsonschema.NewStringLoader(msg)

	// check that user message contains `"initiate":` property with a valid value
	result, err := initiateSchema.Validate(message)

	if err != nil {
		return err
	}
	if !result.Valid() {
		return errors.New(formatJSONError(result))
	}

	parsed := struct {
		Initiate string
	}{}
	err = json.Unmarshal([]byte(msg), &parsed)

	if err != nil {
		return err
	}

	switch parsed.Initiate {
	case "comeOnline":
		r.subRoutine = r.rc.NewComeOnline(r.client, r.hub)
	case "sendConnectionRequest":
		r.subRoutine = r.rc.NewEstablishConnectionToPeer(r.client, r.hub)
	case "sendFriendRequest":
		r.subRoutine = r.rc.NewFriendRequest(r.client, r.hub)
	default:
		return errors.New("routine does not exist")
	}
	return nil
}
