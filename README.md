# HarmonyBackend

## Setup

    go mod download

## Running

    go run .

## Run tests

    go test ./...

## Setup with Docker

To build an image *harmony* and container *harmony0*:

    docker build -t harmony .
    docker run -d -p 8080:8080 --name harmony0 harmony

## Demo usage

Open a websocket connection to testMultiplexWs endpoint, e.g. using `websocat`

    websocat ws://localhost:8080/testMultiplexWs

The first 16 characters of each message is the *transaction id* and uniquely identifies the transaction which represents multiple messages in a sequence. A transaction is an instance of a *routine*.

Example message to start the test routine on a transaction with id `0000000000000000`

    0000000000000000Test

Server replies with

    0000000000000000Please type your name

etc.

See [/routines/demoroutines.go](/routines/demoroutines.go)
