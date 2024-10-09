package model

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

type instantTimeoutRoutine struct {
	routineNumber int
}

func (r *instantTimeoutRoutine) Next(msgType RoutineMsgType, pk *PublicKey, msg string) []RoutineOutput {
	switch msgType {
	case RoutineMsgType_UsrMsg:
		out := MakeRoutineOutput(false, strconv.Itoa(r.routineNumber))
		out.TimeoutEnabled = true
		out.TimeoutDuration = 0 * time.Second
		return []RoutineOutput{out}
	case RoutineMsgType_Timeout:
		// tell it to quit for real.
		out := MakeRoutineOutput(true, `terminate`)
		return []RoutineOutput{out}
	default:
		return []RoutineOutput{}
	}

}

// mock Conn implementation
type mockConn struct {
	outMsgs [][]byte
	fromCl  chan []byte
	done    chan struct{}
}

func (c *mockConn) ReadMessage() (messageType int, p []byte, err error) {
	select {
	case <-c.done:
		return 0, []byte{}, errors.New("connection closed")
	default:
	}
	select {
	case <-c.done:
		return 0, []byte{}, errors.New("connection closed")
	case msg := <-c.fromCl:
		return 0, msg, nil
	}
}
func (c *mockConn) WriteMessage(messageType int, data []byte) error {
	c.outMsgs = append(c.outMsgs, data)
	return nil
}

func TestClient(t *testing.T) {

	t.Run("Correctly times-out routines (see comment)", func(t *testing.T) {

		// Send two messages with the same transaction id in quick succession.
		// The master routine is mocked to timeout instantly and not explicity complete
		// Expect one of the following situations:
		// - 1 routine was initiated, a timeout {terminte, error} sent, message 2 was ignored and an {error} sent
		// - 2 routines were initiated, both received 1 message, and 2 timeout {terminte, error} messages sent

		mockConn := &mockConn{
			outMsgs: make([][]byte, 0),
			fromCl:  make(chan []byte),
			done:    make(chan struct{}),
		}
		client := MakeClient(mockConn)
		mockHub := NewHub()

		var routineInstanceCount = 0
		// use a mock routine that times out instantly, but doesn't explicity complete.
		// for each message received by an instantTimeoutRoutine, it sends its routine number back as a string
		go func() {
			client.Route(mockHub, func() Routine {
				routineInstanceCount += 1
				return &instantTimeoutRoutine{
					routineNumber: routineInstanceCount - 1,
				}
			})
		}()

		idstr := strings.Repeat("0", IDLEN)

		// initiate the routine
		mockConn.fromCl <- []byte(idstr)

		// send another message to the routine
		mockConn.fromCl <- []byte(idstr)

		// wait for 10ms for the messages to come through (sorry, bad practice :( )
		<-time.After(10 * time.Millisecond)

		// close the websocket for reading
		mockConn.done <- struct{}{}

		// convert messages sent to the mock client to string
		// and remove transaction id
		outMsgStrings := make([]string, 0)
		for _, msg := range mockConn.outMsgs {
			outMsgStrings = append(outMsgStrings, string(msg)[IDLEN:])
		}

		// parse and analyze results

		errMsgSchema := func() *gojsonschema.Schema {
			const errorSchemaString = `
				{
					"$schema": "https://json-schema.org/draft/2020-12/schema",
					"type": "object",
					"properties": {
						"error": {
							"type": "string" 
						}
					},
					"required": ["error"],
					"additionalProperties": false
				}
			`
			schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
			schema, _ := gojsonschema.NewSchema(schemaLoader)
			return schema
		}()

		errMsgCount := 0
		for _, str := range outMsgStrings {
			strLoader := gojsonschema.NewStringLoader(str)
			result, err := errMsgSchema.Validate(strLoader)
			if err == nil && result.Valid() {
				errMsgCount += 1
			}
		}

		// errTerminateMsgSchema := func() *gojsonschema.Schema {
		// 	const errorSchemaString = `
		// 		{
		// 			"$schema": "https://json-schema.org/draft/2020-12/schema",
		// 			"type": "object",
		// 			"properties": {
		// 				"terminate": {
		// 					"const":"cancel"
		// 				},
		// 				"error": {
		// 					"type": "string"
		// 				}
		// 			},
		// 			"required": ["terminate"],
		// 			"additionalProperties": false
		// 		}
		// 	`
		// 	schemaLoader := gojsonschema.NewStringLoader(errorSchemaString)
		// 	schema, _ := gojsonschema.NewSchema(schemaLoader)
		// 	return schema
		// }()

		// errTerminateMsgCount := 0
		// for _, str := range outMsgStrings {
		// 	strLoader := gojsonschema.NewStringLoader(str)
		// 	result, err := errTerminateMsgSchema.Validate(strLoader)
		// 	if err == nil && result.Valid() {
		// 		errTerminateMsgCount += 1
		// 	}
		// }

		msgsSentToR0Count := countOccurrences(outMsgStrings, "0")
		msgsSentToR1Count := countOccurrences(outMsgStrings, "1")
		errTerminateMsgCount := countOccurrences(outMsgStrings, "terminate")

		t.Logf("Messages: %v", outMsgStrings)

		switch routineInstanceCount {
		case 1:
			t.Logf("1 routine case.")
			if msgsSentToR0Count != 1 {
				t.Errorf("Expected 1 message to be sent to routine. Got %d", msgsSentToR0Count)
			}
			if errTerminateMsgCount != 1 {
				t.Errorf("Expected 1 error/terminate message to be sent to the user, to signify a timeout. Got %d", errTerminateMsgCount)
			}
			if errMsgCount != 1 {
				t.Errorf("Expected 1 error message to be sent to the user, to signigy a message sent to a terminated transaction. Got %d", errMsgCount)
			}
		case 2:
			t.Logf("2 routine case.")
			if msgsSentToR0Count != 1 {
				t.Errorf("Expected 1 message to be sent to routine 0. Got %d", msgsSentToR0Count)
			}
			if msgsSentToR1Count != 1 {
				t.Errorf("Expected 1 message to be sent to routine 1. Got %d", msgsSentToR1Count)
			}
			if errTerminateMsgCount != 2 {
				t.Errorf("Expected 2 error/terminate message to be sent to the user, to signify timeouts of the 2 routines. Got %d", errTerminateMsgCount)
			}
			if errMsgCount != 0 {
				t.Errorf("Expected 0 error (non-terminate) messages to be sent to the client. Got %d", errMsgCount)
			}
		default:
			t.Errorf("Expected 1 or 2 routines to be created, got %d", routineInstanceCount)
		}

	})
}

func TestSetPublicKey(t *testing.T) {

	pk := (*PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))

	t.Run("Can be used to set the public key", func(t *testing.T) {

		client := &Client{publicKey: nil}
		err := client.SetPublicKey(pk)

		if err != nil {
			t.Errorf(err.Error())
		}
		if client.publicKey != pk {
			t.Errorf("Expected %v got %v", pk, client.publicKey)
		}
	})

	t.Run("Rejects public keys if already set", func(t *testing.T) {
		client := &Client{publicKey: pk}
		err := client.SetPublicKey(pk)

		if err == nil {
			t.Errorf("Expected an error")
		}

	})
}

func countOccurrences[K comparable](slice []K, el K) int {
	count := 0
	for _, item := range slice {
		if item == el {
			count++
		}
	}
	return count
}
