package routines

import (
	"encoding/json"
	"errors"
	"fmt"
	"harmony/backend/model"
	"log/slog"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

type MasterRoutine struct {
	isSubRoutineSet bool
	subRoutine      model.Routine
	rc              RoutineConstructors
	client          *model.Client
	hub             *model.Hub
	logger          *slog.Logger
}

func NewMasterRoutine(client *model.Client, hub *model.Hub, logger *slog.Logger) model.Routine {
	return newMasterRoutineDependencyInj(routineContructorImplementations, client, hub, logger)
}

func newMasterRoutineDependencyInj(rc RoutineConstructors, client *model.Client, hub *model.Hub, logger *slog.Logger) model.Routine {
	return &MasterRoutine{
		rc:     rc,
		client: client,
		hub:    hub,
		logger: logger,
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
var routineNames = []string{"comeOnline", "sendConnectionRequest", "sendFriendRequest", "sendFriendRejection"}

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
		r.logger.Info("bad initiate msg: "+err.Error(), "kind", "ROUTINE_FAIL")
		return err
	}
	if !result.Valid() {
		errStr := formatJSONError(result)
		r.logger.Info("bad initiate msg: "+errStr, "kind", "ROUTINE_FAIL")
		return errors.New(errStr)
	}

	parsed := struct {
		Initiate string
	}{}
	err = json.Unmarshal([]byte(msg), &parsed)

	if err != nil {
		return err
	}

	sublogger := r.logger.With("routine", parsed.Initiate)
	switch parsed.Initiate {
	case "comeOnline":
		r.logger.Info("comeOnline", "kind", "ROUTINE_INIT")
		r.subRoutine = r.rc.NewComeOnline(r.client, r.hub, sublogger)
	case "sendConnectionRequest":
		r.logger.Info("sendConnectionRequest", "kind", "ROUTINE_INIT")
		r.subRoutine = r.rc.NewEstablishConnectionToPeer(r.client, r.hub, sublogger)
	case "sendFriendRequest":
		r.logger.Info("sendFriendRequest", "kind", "ROUTINE_INIT")
		r.subRoutine = r.rc.NewFriendRequest(r.client, r.hub, sublogger)
	case "sendFriendRejection":
		r.logger.Info("sendFriendRejection", "kind", "ROUTINE_INIT")
		r.subRoutine = r.rc.NewFriendRejection(r.client, r.hub, sublogger)
	default:
		panic("routine does not exist")
	}
	return nil
}
