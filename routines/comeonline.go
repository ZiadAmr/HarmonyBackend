package routines

// todo AT MOST ONE instance of this routine for each user should be running at any time.

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

type ComeOnline struct {
	client *model.Client
	hub    *model.Hub
	step   comeOnlineStep
}

type comeOnlineStep int

const ( // enum
	comeOnlineStep_hello comeOnlineStep = iota
	comeOnlineStep_recvPublicKey
)

// constructor
func newComeOnline(client *model.Client, hub *model.Hub) model.Routine {
	return &ComeOnline{
		client: client,
		hub:    hub,
		step:   comeOnlineStep_hello,
	}
}

func (c *ComeOnline) Next(msg string) model.RoutineOutput {

	if isClientCancelMsg(msg) {
		return makeCOOutput(true)
	}

	switch c.step {
	case comeOnlineStep_hello:
		return c.hello()
	case comeOnlineStep_recvPublicKey:
		return c.recvPublicKey(msg)
	}
	panic("Unrecognized step")
}

// send version number
func (c *ComeOnline) hello() model.RoutineOutput {

	if c.client.GetPublicKey() != nil {
		return makeCOOutput(true, MakeJSONError("Public key already set"))
	}
	// set next step
	c.step = comeOnlineStep_recvPublicKey
	// msgs to return to user
	return makeCOOutput(false, `{"version":"`+VERSION+`"}`)
}

func (c *ComeOnline) recvPublicKey(msg string) model.RoutineOutput {
	key, err := parseUserKeyMessage(msg)
	if err != nil {
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}
	_, clientWithKeyAlreadyExists := c.hub.GetClient(*key)
	if clientWithKeyAlreadyExists {
		return makeCOOutput(true, MakeJSONError("Another client already signed in with this public key"))
	}
	c.client.SetPublicKey(key)
	c.hub.AddClient(c.client)
	return makeCOOutput(true, `{"welcome":"welcome","terminate":"done"}`)
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
		return nil, err
	}
	if !result.Valid() {
		return nil, errors.New(formatJSONError(result))
	}

	err = json.Unmarshal([]byte(keyMessageString), &keyMessage)
	if err != nil {
		return nil, err
	}
	keyString := keyMessage.PublicKey
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return nil, errors.New("unable to parse client key")
	}

	return (*model.PublicKey)(key), nil
}

// make ComeOnline output
func makeCOOutput(done bool, msgs ...string) model.RoutineOutput {
	ro := model.MakeRoutineOutput(done, msgs...)
	ro.TimeoutEnabled = true
	ro.TimeoutDuration = timeout
	return ro
}
