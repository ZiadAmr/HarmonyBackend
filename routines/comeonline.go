package routines

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"harmony/backend/model"
	"strconv"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

const timeout = 30 * time.Second

type step int

const ( // enum (weird syntax, don't worry about it)
	failed step = iota
	hello
	recvPublicKey
	done
)

func (r *RoutinesDefn) ComeOnline(client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string, kill chan struct{}) {
	comeOnlineDependencyInj(timeout, client, hub, fromCl, toCl, errCl, kill)
}

func comeOnlineDependencyInj(timeout time.Duration, client *model.Client, hub *model.Hub, fromCl chan string, toCl chan string, errCl chan string, kill chan struct{}) {
	if client.GetPublicKey() != nil {
		errCl <- "public key already set"
		return
	}

	// variable for the main loop
	nextStep := hello
	h := comeOnlineRoutineHandler{
		client: client,
		hub:    hub,
		toCl:   toCl,
	}

	for {
		if nextStep == done || nextStep == failed {
			return
		}

		select {
		case <-kill:
			return

		case <-time.After(timeout):
			errCl <- "timeout"
			return

		case msg := <-fromCl:
			// recieved a message from the client. Initiate the next step
			var err error
			nextStep, err = h.step(nextStep, msg)
			if err != nil {
				errCl <- err.Error()
				return
			}
		}

	}
}

type comeOnlineRoutineHandler struct {
	client *model.Client
	hub    *model.Hub
	toCl   chan string
	// could also store other variables we want to be accessible from the steps in here.
}

func (h comeOnlineRoutineHandler) step(currentStep step, msg string) (step, error) {
	switch currentStep {
	case hello:
		return h.helloStep()
	case recvPublicKey:
		return h.recvPublicKeyStep(msg)
	}
	panic("step not defined?")
}

func (h comeOnlineRoutineHandler) helloStep() (step, error) {

	// don't care about contents of the initial message msg

	h.toCl <- `{
		"version": "` + VERSION + `"
	}`

	return recvPublicKey, nil
}

func (h comeOnlineRoutineHandler) recvPublicKeyStep(keyMessageString string) (step, error) {

	publicKey, err := parseUserKeyMessage(keyMessageString)
	if err != nil {
		return failed, err
	}

	// check that the user is not already signed in on another client
	_, alreadySignedIn := h.hub.GetClient(*publicKey)

	if alreadySignedIn {
		return failed, errors.New("another client already signed in with this public key")
	}

	h.client.SetPublicKey(publicKey)
	h.hub.AddClient(h.client)

	h.toCl <- `{
		"welcome": "welcome",
		"terminate": "done"
	}`

	return done, nil

}

var userKeyMessageSchema = func() *gojsonschema.Schema {
	schemaLoader := gojsonschema.NewStringLoader(`
	{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"publicKey": {
				"type":"string",
				"pattern": "^[0123456789abcdef]{` + strconv.Itoa(model.KEYLEN*2) + `}$"
			}
		},
		"required": ["publicKey"],
		"additionalProperties": false
	}
	`)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

// convert the raw json to a public key
func parseUserKeyMessage(keyMessageString string) (*model.PublicKey, error) {
	keyMessage := struct {
		PublicKey string
	}{}

	messageLoader := gojsonschema.NewStringLoader(keyMessageString)
	result, err := userKeyMessageSchema.Validate(messageLoader)

	if err != nil {
		return nil, errors.New("unable to parse client message")
	}
	if !result.Valid() {
		return nil, errors.New(formatJSONError(result))
	}

	err = json.Unmarshal([]byte(keyMessageString), &keyMessage)
	if err != nil {
		return nil, errors.New("unable to parse client message")
	}
	keyString := keyMessage.PublicKey
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return nil, errors.New("unable to parse client key")
	}

	return (*model.PublicKey)(key), nil
}
