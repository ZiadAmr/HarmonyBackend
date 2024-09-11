package routines

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"harmony/backend/model"
	"strconv"

	"github.com/xeipuuv/gojsonschema"
)

func (r *RoutinesDefn) ComeOnline(client *model.Client, fromCl chan string, toCl chan string, errCl chan string) {

	// initial message. Don't care about anything in here.
	<-fromCl

	toCl <- `{
		"version": "` + VERSION + `"
	}`

	// get user key
	keyMessageString := <-fromCl
	publicKey, err := parseUserKeyMessage(keyMessageString)
	if err != nil {
		errCl <- err.Error()
		return
	}
	client.SetPublicKey(publicKey)

	toCl <- `{
		"welcome": "welcome",
		"terminate": "done"
	}`
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
