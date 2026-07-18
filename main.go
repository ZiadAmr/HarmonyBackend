package main

import (
	"fmt"
	"strings"

	"harmony/backend/model"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/alexflint/go-arg"
)

// used to upgrade HTTP protocol to websocket protocol
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// pointers to online clients stored in here
var hub *model.Hub

func main() {

	args := struct {
		Hostnames string `arg:"-d" default:"0.0.0.0" help:"comma-separated list of hostnames by which this server can be accessed" placeholder:"DOMS"`
		Port      uint16 `arg:"-p" default:"10080" help:"port to listen on" placeholder:"N"`
	}{}
	arg.MustParse(&args)
	hostnames := strings.Split(args.Hostnames, ",")
	for i, d := range hostnames {
		hostnames[i] = strings.TrimSpace(d)
	}

	hub = model.NewHub(hostnames)

	router := gin.Default()

	// Main entry point
	router.GET("/ws", handleWs)

	router.Run("0.0.0.0:" + fmt.Sprintf("%d", args.Port))
}
