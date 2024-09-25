// useful info
// https://aditechsavvyblogs.hashnode.dev/mastering-gorilla-websockets

package main

import (
	"fmt"

	// "time"

	"harmony/backend/model"
	"harmony/backend/routines"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// endpoints
// get test
func getTest(c *gin.Context) {
	fmt.Println("Recieved GET /test")
}

// used to upgrade HTTP protocol to websocket protocol
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var hub = model.NewHub()

func main() {

	router := gin.Default()

	// Main entry point
	router.GET("/ws", func(c *gin.Context) {

		// upgrade to websocket protocol
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		// defer means it executes after the function returns.
		defer func() {
			// TODO: remove the client from the hub
			// TODO: delete the client
			conn.Close()
		}()

		client := model.MakeClient(conn)
		client.Route(func(fromCl chan string, toCl chan string) {
			routines.MasterRoutine(&client, hub, fromCl, toCl)
		})

	})

	router.GET("/test", getTest)

	// demo functions
	router.GET("/testMultiplexWs", func(c *gin.Context) {

		// upgrade to websocket protocol
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		// defer means it executes after the function returns.
		defer conn.Close()
		defer fmt.Println("Client has closed the websocket connection.")

		client := model.MakeClient(conn)
		client.Route(func(fromCl, toCl chan string) {
			routines.MasterTestRoutine(fromCl, toCl, &client)
		})
	})

	router.Run("0.0.0.0:8080")
}
