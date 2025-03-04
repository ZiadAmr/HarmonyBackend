package routines

// todo AT MOST ONE instance of this routine for each user should be running at any time.

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"harmony/backend/model"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

const timeout = 30 * time.Second

type ComeOnline struct {
	client     *model.Client
	hub        *model.Hub
	step       comeOnlineStep
	randMsgGen RandomMessageGenerator

	signThis         string
	publicKey        *model.PublicKey
	ed25519PublicKey *ed25519.PublicKey
}

type RandomMessageGenerator interface {
	GetMessage() (string, error)
}

type RandomMessageGeneratorImpl struct{}

// generate a random string for clients to sign
func (r RandomMessageGeneratorImpl) GetMessage() (string, error) {
	buf := make([]byte, 128)
	_, err := rand.Read(buf)
	if err != nil {
		return "", errors.New("internal server error generating a random string")
	}
	// encode random bytes in base64
	randStr := base64.StdEncoding.EncodeToString(buf)
	return randStr, nil
}

type comeOnlineStep int

const ( // enum
	comeOnlineStep_hello comeOnlineStep = iota
	comeOnlineStep_recvPublicKey
	comeOnlineSign_recvSignature
)

// constructor
func newComeOnline(client *model.Client, hub *model.Hub) model.Routine {
	return newComeOnlineDependencyInj(client, hub, RandomMessageGeneratorImpl{})
}

func newComeOnlineDependencyInj(client *model.Client, hub *model.Hub, randMsgGen RandomMessageGenerator) model.Routine {
	return &ComeOnline{
		client:     client,
		hub:        hub,
		randMsgGen: randMsgGen,
		step:       comeOnlineStep_hello,
	}
}

func (c *ComeOnline) Next(args model.RoutineInput) []model.RoutineOutput {

	switch args.MsgType {
	case model.RoutineMsgType_ClientClose:
		return []model.RoutineOutput{}
	case model.RoutineMsgType_Timeout:
		return makeCOOutput(true, MakeJSONError("timeout"))
	case model.RoutineMsgType_UsrMsg:
		if isClientCancelMsg(args.Msg) {
			return makeCOOutput(true)
		}
		switch c.step {
		case comeOnlineStep_hello:
			return c.hello()
		case comeOnlineStep_recvPublicKey:
			return c.recvPublicKey(args.Msg)
		case comeOnlineSign_recvSignature:
			return c.recvSignature(args.Msg)
		}
		panic("unrecognized step")
	}
	panic("unrecognized message type")

}

// send version number
func (c *ComeOnline) hello() []model.RoutineOutput {

	if c.client.GetPublicKey() != nil {
		return makeCOOutput(true, MakeJSONError("Public key already set"))
	}
	// set next step
	c.step = comeOnlineStep_recvPublicKey
	// msgs to return to user
	return makeCOOutput(false, `{"version":"`+VERSION+`"}`)
}

func (c *ComeOnline) recvPublicKey(msg string) []model.RoutineOutput {
	key, keyBytes, err := parseUserKeyMessage(msg)
	if err != nil {
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}
	_, clientWithKeyAlreadyExists := c.hub.GetClient(*key)
	if clientWithKeyAlreadyExists {
		return makeCOOutput(true, MakeJSONError("Another client already signed in with this public key"))
	}

	c.publicKey = key
	c.ed25519PublicKey = keyBytes

	// generate a random message for the client to sign with their private key
	c.signThis, err = c.randMsgGen.GetMessage()
	if err != nil {
		makeCOOutput(true, MakeJSONError(err.Error()))
	}
	signThisMsgData := struct {
		SignThis string `json:"signThis"`
	}{}
	signThisMsgData.SignThis = c.signThis
	signThisMsgStr, _ := json.Marshal(signThisMsgData)

	// set next step
	c.step = comeOnlineSign_recvSignature

	return makeCOOutput(false, (string)(signThisMsgStr))
}

func (c *ComeOnline) recvSignature(msg string) []model.RoutineOutput {

	// parse signature to byte array
	sig, err := parseUserSignatureMessage(msg)
	if err != nil {
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}

	// verify signature
	valid := ed25519.Verify(*c.ed25519PublicKey, []byte(c.signThis), sig)
	if !valid {
		return makeCOOutput(true, MakeJSONError("Invalid signature"))
	}

	// add to hub
	err = c.hub.AddClient(*c.publicKey, c.client)
	if err != nil {
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}

	// set client pk
	c.client.SetPublicKey(c.publicKey)

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
				"pattern": "` + publicKeyPattern + `"
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
func parseUserKeyMessage(keyMessageString string) (*model.PublicKey, *ed25519.PublicKey, error) {
	// verify json
	messageLoader := gojsonschema.NewStringLoader(keyMessageString)
	result, err := userKeyMessageSchema.Validate(messageLoader)
	if err != nil {
		return nil, nil, err
	}
	if !result.Valid() {
		return nil, nil, errors.New(formatJSONError(result))
	}

	// parse json
	keyMessage := struct {
		PublicKey string `json:"publicKey"`
	}{}
	err = json.Unmarshal([]byte(keyMessageString), &keyMessage)
	if err != nil {
		return nil, nil, err
	}

	// convert key to model.publicKey
	keyString := keyMessage.PublicKey
	key, err := parsePublicKey(keyString)
	if err != nil {
		return nil, nil, errors.New("unable to parse public key")
	}

	// decode base64
	keyDER, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, nil, err
	}

	// parse DER
	keyDecoded, err := x509.ParsePKIXPublicKey(keyDER)
	if err != nil {
		return nil, nil, errors.New("public key is not ed25519")
	}

	// assert ed25519 and return
	if keyDecoded, ok := keyDecoded.(ed25519.PublicKey); ok {
		return (*model.PublicKey)(key), &keyDecoded, nil
	} else {
		return nil, nil, errors.New("public key is not ed25519")
	}

}

var userSignatureMessageSchema = func() *gojsonschema.Schema {
	schemaLoader := gojsonschema.NewStringLoader(`
	{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"signature": {
				"type":"string",
				"pattern": "` + signaturePattern + `"
			}
		},
		"required": ["signature"],
		"additionalProperties": false
	}
	`)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

func parseUserSignatureMessage(signatureMessageString string) ([]byte, error) {

	// validate against json schema
	messageLoader := gojsonschema.NewStringLoader(signatureMessageString)
	result, err := userSignatureMessageSchema.Validate(messageLoader)
	if err != nil {
		return nil, err
	}
	if !result.Valid() {
		return nil, errors.New(formatJSONError(result))
	}

	// parse msg
	usrMsg := struct {
		Signature string `json:"signature"`
	}{}
	err = json.Unmarshal([]byte(signatureMessageString), &usrMsg)
	if err != nil {
		return nil, err
	}

	// decode base64 signature
	sig, err := base64.StdEncoding.DecodeString(usrMsg.Signature)
	if err != nil {
		return nil, err
	}

	return sig, nil

}

// make ComeOnline output
func makeCOOutput(done bool, msgs ...string) []model.RoutineOutput {
	ro := model.MakeRoutineOutput(done, msgs...)
	ro.TimeoutEnabled = true
	ro.TimeoutDuration = timeout
	return []model.RoutineOutput{ro}
}
