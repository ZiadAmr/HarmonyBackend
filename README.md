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

## Docs

See [/docs](/docs)

## Demo usage

Open a websocket connection to `ws` endpoint, e.g. using `websocat`

    websocat ws://localhost:8080/ws

The first 16 characters of each message is the *transaction id* and uniquely identifies the transaction which represents multiple messages in a sequence. A transaction is an instance of a *routine*.

Example message to start the `comeOnline` on a transaction with id `0000000000000000`

    0000000000000000{"initiate","comeOnline"}

Server replies with

    0000000000000000{"version":"0.0"}

etc.




