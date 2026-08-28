package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"harmony/backend/model"
	"harmony/backend/version"

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
var logger *slog.Logger

func main() {

	args := struct {
		Hostnames string `arg:"-d" default:"0.0.0.0" help:"comma-separated list of hostnames by which this server can be accessed" placeholder:"DOMS"`
		Port      uint16 `arg:"-p" default:"10080" help:"port to listen on" placeholder:"N"`
		Verbose   bool   `arg:"-v" help:"output everything that clients are doing to stdout"`
		Version   bool   `arg:"-V" help:"display version info and exit"`
	}{}
	arg.MustParse(&args)

	if args.Version {
		fmt.Printf("HarmonyBackend version: %s\n", version.VERSION)
		fmt.Printf("Client/Server API version: %s\n", version.SERVER_API_VERSION)
		os.Exit(0)
	}

	hostnames := strings.Split(args.Hostnames, ",")
	for i, d := range hostnames {
		hostnames[i] = strings.TrimSpace(d)
	}

	hub = model.NewHub(hostnames)
	if args.Verbose {
		logger = slog.New(newHarmonyTextHandler(os.Stdout, true))
	} else {
		logger = slog.New(newHarmonyTextHandler(io.Discard, false))
	}

	router := gin.Default()

	// Main entry point
	router.GET("/ws", handleWs)

	router.Run("0.0.0.0:" + fmt.Sprintf("%d", args.Port))
}
