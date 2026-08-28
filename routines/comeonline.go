package routines

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"harmony/backend/model"
	"harmony/backend/version"
	"log/slog"
	"slices"
	"time"

	"github.com/gibson042/canonicaljson-go"
	"github.com/xeipuuv/gojsonschema"
)

const timeout = 30 * time.Second

type ComeOnline struct {
	client         *model.Client
	hub            *model.Hub
	step           comeOnlineStep
	randMsgGen     RandomMessageGenerator
	currentTimeGen CurrentTimeGenerator
	logger         *slog.Logger

	challenge        string
	publicKey        *model.PublicKey
	ed25519PublicKey *ed25519.PublicKey

	holdsComeOnlineLock bool
}

type RandomMessageGenerator interface {
	GetMessage() (string, error)
}

type CurrentTimeGenerator interface {
	GetCurrentTime() string
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

type CurrentTimeGeneratorImpl struct{}

func (c CurrentTimeGeneratorImpl) GetCurrentTime() string {
	return time.Now().Format(time.RFC3339)
}

type comeOnlineStep int

const ( // enum
	comeOnlineStep_hello comeOnlineStep = iota
	comeOnlineStep_recvPublicKey
	comeOnlineStep_recvSignature
)

// constructor
func newComeOnline(client *model.Client, hub *model.Hub, logger *slog.Logger) model.Routine {
	return newComeOnlineDependencyInj(client, hub, logger, RandomMessageGeneratorImpl{}, CurrentTimeGeneratorImpl{})
}

func newComeOnlineDependencyInj(client *model.Client, hub *model.Hub, logger *slog.Logger, randMsgGen RandomMessageGenerator, currentTimeGen CurrentTimeGenerator) model.Routine {
	return &ComeOnline{
		client:         client,
		hub:            hub,
		randMsgGen:     randMsgGen,
		currentTimeGen: currentTimeGen,
		step:           comeOnlineStep_hello,
		logger:         logger,
	}
}

func (c *ComeOnline) Next(args model.RoutineInput) []model.RoutineOutput {

	// attempt to get lock
	if !c.holdsComeOnlineLock {
		succeed := c.client.ComeOnlineLock.TryLock()
		if !succeed {
			c.logger.Info("concurrent comeOnline", "kind", "ROUTINE_FAIL")
			return makeCOOutput(true, MakeJSONError("Another comeOnline routine is in progress"))
		}
		c.holdsComeOnlineLock = true
	}

	nextResult := c.safeNext(args)

	// check if lock needs to be released
	if args.MsgType == model.RoutineMsgType_ClientClose || (len(nextResult) > 0 && nextResult[0].Done) {
		c.client.ComeOnlineLock.Unlock()
		c.holdsComeOnlineLock = false
	}

	return nextResult
}

/**Called by Next() only if the lock is obtained.*/
func (c *ComeOnline) safeNext(args model.RoutineInput) []model.RoutineOutput {

	switch args.MsgType {
	case model.RoutineMsgType_ClientClose:
		c.logger.Info("pk A close", "kind", "ROUTINE_FAIL")
		return []model.RoutineOutput{}
	case model.RoutineMsgType_Timeout:
		c.logger.Info("pk A timeout", "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("timeout"))
	case model.RoutineMsgType_UsrMsg:
		if isClientCancelMsg(args.Msg) {
			c.logger.Info("pk A cancel", "kind", "ROUTINE_FAIL")
			return makeCOOutput(true)
		}
		switch c.step {
		case comeOnlineStep_hello:
			return c.hello()
		case comeOnlineStep_recvPublicKey:
			return c.recvPublicKey(args.Msg)
		case comeOnlineStep_recvSignature:
			return c.recvSignature(args.Msg)
		}
		panic("unrecognized step")
	}
	panic("unrecognized message type")

}

// send version number
func (c *ComeOnline) hello() []model.RoutineOutput {

	if c.client.GetPublicKey() != nil {
		c.logger.Info("pk already set", "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("public key already set"))
	}
	// set next step
	c.step = comeOnlineStep_recvPublicKey
	// msgs to return to user
	return makeCOOutput(false, `{"version":"`+version.SERVER_API_VERSION+`"}`)
}

func (c *ComeOnline) recvPublicKey(msg string) []model.RoutineOutput {
	key, keyBytes, err := parseUserKeyMessage(msg)
	if err != nil {
		c.logger.Info("bad public key msg: "+err.Error(), "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}
	_, clientWithKeyAlreadyExists := c.hub.GetClient(*key)
	if clientWithKeyAlreadyExists {
		c.logger.Info("client with key "+string(*key)+" already exists", "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("Another client already signed in with this public key"))
	}

	c.publicKey = key
	c.ed25519PublicKey = keyBytes

	// generate a random message for the client to sign with their private key
	c.challenge, err = c.randMsgGen.GetMessage()
	if err != nil {
		makeCOOutput(true, MakeJSONError(err.Error()))
	}
	challengeMsgData := struct {
		Challenge string `json:"challenge"`
	}{}
	challengeMsgData.Challenge = c.challenge
	challengeMsgStr, _ := json.Marshal(challengeMsgData)

	// set next step
	c.step = comeOnlineStep_recvSignature

	return makeCOOutput(false, (string)(challengeMsgStr))
}

func (c *ComeOnline) recvSignature(msg string) []model.RoutineOutput {

	// parse signature to byte array
	msgObj, err := parseUserSignatureMessage(msg)
	if err != nil {
		c.logger.Info("error parsing signature message: "+err.Error(), "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}

	// check challenge matches
	if msgObj.Payload.Challenge != c.challenge {
		c.logger.Info("challenge does not match", "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("Challenge does not match"))
	}

	// check hostname is allowed
	if !(slices.Contains(c.hub.AllowedHostnames, msgObj.Payload.Hostname) || slices.Contains(c.hub.AllowedHostnames, "0.0.0.0")) {
		c.logger.Info(`hostname "`+msgObj.Payload.Hostname+`" not allowed`, "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("Hostname not allowed"))
	}

	// check client time is no more than 2 seconds different from server time
	serverTimeRFC3339 := c.currentTimeGen.GetCurrentTime()
	serverTime, _ := time.Parse(time.RFC3339, serverTimeRFC3339)
	clientTime, err := time.Parse(time.RFC3339, msgObj.Payload.CurrentTime)
	if err != nil {
		c.logger.Info("could not parse client time: "+err.Error(), "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}
	if serverTime.Sub(clientTime).Abs() > 2*time.Second {
		c.logger.Info("client and server times are not synchronized. Current client time is "+msgObj.Payload.CurrentTime, "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("Server and client times are not synchronized. Current server time is "+serverTimeRFC3339))
	}

	// decode base64 signature
	sig, err := base64.StdEncoding.DecodeString(msgObj.Signature)
	if err != nil {
		c.logger.Info("could not decode signature base64: "+err.Error(), "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}

	// verify signature
	serializedPayload, err := canonicaljson.Marshal(msgObj.Payload)
	if err != nil {
		c.logger.Info(err.Error(), "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}
	valid := ed25519.Verify(*c.ed25519PublicKey, serializedPayload, sig)
	if !valid {
		c.logger.Info("invalid signature", "kind", "ROUTINE_FAIL")
		return makeCOOutput(true, MakeJSONError("Invalid signature"))
	}

	// add to hub
	err = c.hub.AddClient(*c.publicKey, c.client)
	if err != nil {
		return makeCOOutput(true, MakeJSONError(err.Error()))
	}

	// set client pk
	c.client.SetPublicKey(c.publicKey)

	c.logger.Info(string(*c.publicKey), "kind", "ROUTINE_SUCCEED")

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
			"payload": {
				"type":"object",
				"properties": {
					"challenge": {
						"type": "string"
					},
					"hostname": {
						"type": "string" 
					},
					"purpose": {
						"const": "comeOnline" 
					},
					"currentTime": {
						"type": "string",
						"pattern": "` + rfc3339TimePattern + `"
					}
				},
				"required": ["challenge", "hostname", "purpose", "currentTime"],
				"additionalProperties": false
			},
			"signature": {
				"type":"string",
				"pattern": "` + signaturePattern + `"
			}
		},
		"required": ["payload", "signature"],
		"additionalProperties": false
	}
	`)
	schema, _ := gojsonschema.NewSchema(schemaLoader)
	return schema
}()

type UserSignatureMessage struct {
	Payload struct {
		Challenge   string `json:"challenge"`
		Hostname    string `json:"hostname"`
		Purpose     string `json:"purpose"`
		CurrentTime string `json:"currentTime"`
	} `json:"payload"`
	Signature string `json:"signature"`
}

func parseUserSignatureMessage(signatureMessageString string) (*UserSignatureMessage, error) {

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
	usrMsg := UserSignatureMessage{}
	err = json.Unmarshal([]byte(signatureMessageString), &usrMsg)
	if err != nil {
		return nil, err
	}

	// // decode base64 signature
	// sig, err := base64.StdEncoding.DecodeString(usrMsg.Signature)
	// if err != nil {
	// 	return nil, err
	// }

	return &usrMsg, nil

}

// make ComeOnline output
func makeCOOutput(done bool, msgs ...string) []model.RoutineOutput {
	ro := model.MakeRoutineOutput(done, msgs...)
	ro.TimeoutEnabled = true
	ro.TimeoutDuration = timeout
	return []model.RoutineOutput{ro}
}
