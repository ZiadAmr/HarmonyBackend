package main

import (
	"harmony/backend/model"
	"harmony/backend/routines"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var PING_MESSAGE = []byte("p")

const pingPeriod = 10 * time.Second
const pongMaxWait = pingPeriod / 2

func handleWs(c *gin.Context) {
	// upgrade to websocket protocol
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// heartbeat
	// if we haven't received a ping or a ping for 10 seconds then send a ping. If no pong response in 5 seconds then disconnect.
	receivedPingPong := make(chan struct{})
	defer close(receivedPingPong)

	conn.SetPingHandler(func(message string) error {
		receivedPingPong <- struct{}{}
		_ = conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(time.Second))
		return nil
	})
	conn.SetPongHandler(func(appData string) error {
		receivedPingPong <- struct{}{}
		return nil
	})

	const timeBetweenPings = 10 * time.Second
	const timeToWaitForPong = 5 * time.Second
	const pingMaxSendDelay = 1 * time.Second

	go func() {

		check := time.NewTicker(time.Duration(float32(timeBetweenPings) * /*±10%*/ (0.9 + rand.Float32()*0.2)))
		close := time.NewTimer( /*arbitrary large number*/ time.Hour)
		close.Stop()

		for {
			select {
			case _, ok := <-receivedPingPong:
				if !ok {
					return
				}
				// reset timers
				check.Reset(time.Duration(float32(timeBetweenPings) * /*±10%*/ (0.9 + rand.Float32()*0.2)))
				if !close.Stop() {
					select {
					case <-close.C:
					default:
					}
				}

			case <-check.C:
				// no ping or pong received for 10 seconds
				conn.WriteControl(websocket.PingMessage, PING_MESSAGE, time.Now().Add(pingMaxSendDelay))
				// set timer for pong response
				if !close.Stop() {
					select {
					case <-close.C:
					default:
					}
				}
				close.Reset(timeToWaitForPong)

			case <-close.C:
				// no pong response - close connection
				conn.Close()
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
