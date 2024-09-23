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
		defer conn.Close()

		// client := model.MakeClient(conn)
		// client.Route(routines.MasterRoutine)
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

		client := model.MakeClient(conn)
		client.Route(routines.MasterTestRoutine)
	})

	router.Run("0.0.0.0:8080")
}
