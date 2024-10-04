// useful info
// https://aditechsavvyblogs.hashnode.dev/mastering-gorilla-websockets

package main

import (
	"fmt"

	// "time"

	"harmony/backend/model"

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

// pointers to online clients stored in here
var hub = model.NewHub()

func main() {

	router := gin.Default()

	// Main entry point
	router.GET("/ws", handleWs)

	router.GET("/test", getTest)

	router.Run("0.0.0.0:8080")
}
