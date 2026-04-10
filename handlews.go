package main

import (
	"harmony/backend/model"
	"harmony/backend/routines"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var PING_MESSAGE = []byte("p")

const pingPeriod = 10 * time.Second
const pingMaxSendDelay = 1 * time.Second
const pongMaxWait = pingPeriod / 2

func handleWs(c *gin.Context) {
	// upgrade to websocket protocol
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// heartbeat
	receivedPong := make(chan struct{}) // close of this channel causes ping pong loop to exit
	defer close(receivedPong)
	conn.SetPongHandler(func(appData string) error {
		// possibly a panic here if the behaviour of gorilla websocket is to continue calling this handler fn after conn.Close() (which leads to receivedPong chan closing)
		receivedPong <- struct{}{}
		return nil
	})
	// defer conn.SetPongHandler(func(appData string) error { return nil }) // remove handler
	go func() {
		pongTimeout := time.NewTimer(time.Hour)
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.WriteControl(websocket.PingMessage, PING_MESSAGE, time.Now().Add(pingMaxSendDelay))
				// stop timeout and drain channel
				if !pongTimeout.Stop() {
					select {
					case <-pongTimeout.C:
					default:
					}
				}
				pongTimeout.Reset(pongMaxWait)

			case <-pongTimeout.C:
				// pong was not received within the time limit
				// disconnected
				conn.Close()
			case _, ok := <-receivedPong:
				if !ok {
					return
				}
				// stop timeout and drain channel
				if !pongTimeout.Stop() {
					select {
					case <-pongTimeout.C:
					default:
					}
				}

			}
		}
	}()

	createAndRouteClient(conn)

}

func createAndRouteClient(conn model.Conn) {

	client := model.MakeClient(conn)

	// delete client when done (closed connection)
	defer func() {
		pk := client.GetPublicKey()
		if pk != nil {
			// client was added to the hub
			err := hub.DeleteClient(*pk)
			if err != nil {
				panic(err)
			}
		}
	}()

	client.Route(hub, func() model.Routine {
		return routines.NewMasterRoutine(&client, hub)
	})

}
