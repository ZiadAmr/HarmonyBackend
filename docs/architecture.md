# Architecture

How the various parts of the code fit together.

## main.go

Listens for, and handles websocket connection requests.

## model

Contains **client**, **transaction**, and **hub**.

The **client** contains the `Client` struct and several member functions. The most important is `Route`, which is the only function that can read and write from the websocket.

**Transactions** are used to multiplex different routines over the same websocket connection. Each message sent or received by the websocket begins with a *transaction ID*. If message with an unrecognized transaction ID is received, a new transaction is created with that ID. Creating and deleting transactions is handled in the `Route` function of the `Client` struct.

The **hub** contains pointers to all clients that have provided their public key.

## routines

A routine achieves a single task, often comprising of multiple back-and-forth communications with the client. For instance, **establish connection to peer** is a routine that facilitates the creation of a WebRTC connection between the client and another peer.

When a new transaction is initiated by the client, the **master routine** is called with 2 channels, `fromCl` and `toCl`. Messages from the client are sent to `fromCl`, and the master routine can send messages to the client using `toCl`. These messages do not include the transaction ID.

The API specifies that clients should include the name of routine they want to call in the first message of a transaction, in a JSON string of the form `{"initiate": "..."}`. Therefore, the master routine parses this first message and calls an appropriate routine handler function, forwarding this message and all subsequent messages to that function.