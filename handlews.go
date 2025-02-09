package main

import (
	"harmony/backend/model"
	"harmony/backend/routines"

	"github.com/gin-gonic/gin"
)

func handleWs(c *gin.Context) {
	// upgrade to websocket protocol
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	// defer means it executes after the function returns.
	defer conn.Close()

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
