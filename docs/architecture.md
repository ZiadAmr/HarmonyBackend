# Architecture

## Routines and Transactions

**Routine:** A routine is a procedure that achieves a single task, often comprising of multiple back-and-forth communications with 1 or more clients. In the code, routines are structs that implement the [`Routine`](/model/routineinterface.go) interface:

```golang
type Routine interface {
	Next(args RoutineInput) []RoutineOutput
}
```

> Example: [`ComeOnline`](/routines/comeonline.go) is a routine that allows a client to set their public key.

> ⚠️ Not to be confused with the term *goroutine*, which is a feature of the Go language.

**Transaction:** an active instance of a routine. In the code, a [`transaction`](/model/transaction.go) is a struct, wrapping the `Routine` struct and other variables and channels for communication.

**Transaction socket:** The means by which a client interacts with a transaction. Transaction sockets are assosiated with *transaction (socket) IDs*, which determine the prefix used when sending or receiving messages. Transaction IDs are 16 chars/bytes. In the code, a [`transactionSocket`](/model/transaction.go) is a struct wrapping a `transaction`, and one is owned by each client interacting with the transaction.

## Example

A client sends the following message to the server through their websocket: 

(**Client ➜ Server**)

    0000000000000000{"initiate","comeOnline"}

The server checks if the client has a transaction socket with id `"0000000000000000"`. If so, it passes the message `{"initiate","comeOnline"}` to the corresponding routine. If not, the server creates a new [`MasterRoutine`](/routines/master.go) (implements `Routine`), `transaction`, and `transactionSocket` with the transaction socket id `"0000000000000000"`.

The message is sent to the master routine by a call to `.Next(...)` (as defined in the `Routine` interface). Assuming this is the first message of the transaction, the `MasterRoutine` parses the message and sees that the user wants to initiate the comeOnline routine. It creates a `ComeOnline` struct (also implementing `Routine`), and forwards this first message and all subsequent messages.

Within the return value of `.Next(...)`, `ComeOnline` orders a message `{"version":"0.0"}` to be sent back to the client. The server prepends the transaction socket id to the message, and sends the following: 

(**Server ➜ Client**)

    0000000000000000{"version":"0.0"}

The client replies with a message containing their public key:

(**Client ➜ Server**)

    0000000000000000{"publicKey": "cffd10babed1182e7d8e6cff845767eeae4508aa13cd00379233f57f799dc18c1eefd35b51db36e3da4770737a3f8fe75eda0cd3c48f23ea705f3234b0929f9e"}

Another call to `.Next(...)` is made. In the return value, in addition to the message to the client, ComeOnline orders that the transaction socket should be closed (by setting `done` to `true` in the [`RoutineOutput`](/model/routineinterface.go))

(**Server ➜ Client**)

    0000000000000000{"terminate":"done"}


## Model

**Client:** A [`Client`](/model/client.go) is a struct maintained for each online client, online meaning that it has a websocket connection to the server. 

**Hub:** The [`Hub`](/model/hub.go) contains pointers to all clients with set public keys.

## Routine interface

> ℹ️ Note: the routine interace is defined in the [model](/model/) package. When accessing these identifiers from the [routines](/routines/) package, `model.` must be prepended.

Below is the routine interface as of commit [c149263a787514163d4da9ea715d10764bd6437d](https://github.com/ZiadAmr/HarmonyBackend/tree/c149263a787514163d4da9ea715d10764bd6437d).

```golang
type Routine interface {
	Next(args RoutineInput) []RoutineOutput
}
```

`RoutineInput` is the type:

```golang
type RoutineInput struct {
	MsgType RoutineMsgType
	// public key is nil if unset.
	Pk      *PublicKey
	// can be ignored if MsgType is not RoutineMsgType_UsrMsg.
	Msg string
}
```

`MsgType` is a value of an enum `RoutineMsgType`, which can take the values `RoutineMsgType_UsrMsg`, `RoutineMsgType_Timeout`, and `RoutineMsgType_ClientClose`.

The routine should return a list of `RoutineOutput`, which contains at most 1 `RoutineOutput` for each client (but each output can contain multiple messages). If a `RoutineOutput` is for a client that does not have a transaction socket, then one is created with a new id. `RoutineOutput`s can also set a timeout timer, after which, if no messsage is received from the client, the routine will get another input of `MsgType` `RoutineMsgType_Timeout`.

`RoutineOutput` is the type:

```golang
type RoutineOutput struct {
	// Public key of the client to send messages to.
	// Nil to reply to the client that sent the message.
	Pk *PublicKey
	// 0 or more messages to send to the client
	Msgs []string
	// whether the routine should no longer accept messages from the client.
	// routine should NOT send any more messages after sending Done=true, or receiving a msg of msgType RoutineMsgType_ClientClose. This could result in a panic().
	Done bool
	// if no message is received within the timeout then the routine gets a .Next() with message type RoutineMsgType_Timeout
	// and can deal with it however it wants (e.g. by returning a RoutineOutput with done=true)
	TimeoutDuration time.Duration
	TimeoutEnabled  bool
}
```

## Goroutines and communication

Below is am example diagram showing the structure of the goroutines, and channel communication between them. In the example there are 2 clients and 2 transactions. Client `c0` can interact with the first transaction `t0`, and both clients can interact with `t1`.

![Goroutines and comminication diagram](internal%20communication%20pattern.png "Goroutines and comminication diagram")

### Types of goroutine in this application

1. **Route Client (RC) goroutine**.

    - **Role:** Unique for each client. This goroutine is initiated by the websocket handler when a client opens a websocket. After some initial work and the creation of a `Client` struct, this goroutine spends its time in `(*Client).Route()`, and forwards incoming messages on the websocket to appropriate RTS goroutines.
    - **Started by:** client opening a websocket connection
    - **Reads from:**
        - the websocket
    - **Writes to:**
        - `transactionSocket.clientMsgChan`
        - `transactionSocket.clientCloseChan`
    - **Closes channels:**
        - `transactionSocket.clientMsgChan` after the channel has been added to the dangling channels list by RTS
        - `transactionSocket.clientCloseChan` after the channel has been added to the dangling channels list by RTS
    - **Terminated by:** websocket closing

2. **Route Transaction Socket (RTS) goroutine**

    - **Role:** Unique for each transaction socket. Runs in `(*Client).routeTransactionSocket()`. Recieves messages from the RC goroutine and forwards them to the RT goroutine. Recieves routine output messages from the RT goroutine, updates its state and sends messages to the client. Responsible for deleting the transaction socket when appropriate, including moving `transactionSocket.clientMsgChan` and `transactionSocket.clientCloseChan` to respective 'dangling channels' lists to be closed by RC.
    - **Started by:**
        - RC goroutine, when receiving a message from a client with transaction socket ID not referencing an existing transaction socket
        - RT goroutine, when a routine output is to be send to a client that has not yet had have a socket for the transaction
    - **Reads from:**
        - `transactionSocket.clientMsgChan`
        - `transactionSocket.clientCloseChan`
        - `transactionSocket.roChan`
    - **Writes to:**
        - `transaction.riChan` (many-to-one)
        - the websocket
    - **Closes:**
        - `transaction.riChan` when the transaction socket is deleted, if this goroutine is the last writer
    - **Terminated by:** All channels that it reads from being closed.

3. **Route Transaction (RT) goroutine**

    - **Role:** Unique for each transaction. Runs in `(*transaction).route()`. Calls `.Step()` on the routine, and distributes the outputs.
    - **Started by:** RC goroutine, when a client starts a new transaction
    - **Reads from:**
        - `transaction.riChan`
    - **Writes to:**
        - `transactionSocket.roChan` of every transaction socket
    - **Closes:** 
        - `transactionSocket.roChan` of a socket when specified by the routine, or when receiving a message that the respective client has disconnected.
    - **Terminated by:** `transaction.riChan` being closed.

### Purpose of Channels

Each `transaction` owns this channel:

```golang
riChan chan routineInputWrapper
```

- `riChan` ("routine input channel") is sent messages from all RTS goroutines that have a socket on this transaction. The channel has a buffer to create a fifo queue for incoming messages. If this buffer is filled then `RoutineMsgType_UsrMsg` messages are rejected in RTS, but other messages are not.

`routineInputWrapper` is the type:

```golang
type routineInputWrapper struct {
	args         RoutineInput
	senderRoChan chan RoutineOutput
}
```

`senderRoChan` is needed in case the sender client has not yet set their public key, in which case RT has no way to look up the sender's `roChan`.

Each `transactionSocket` owns these channels:


```golang
clientMsgChan   chan string
clientCloseChan chan struct{} // empty struct - no data. just used for signalling
roChan          chan RoutineOutput
```

- `clientMsgChan` carries messages from the user as strings, with the transaction socket ID having been removed. It is written to by RC, and read from by RTS.
- `clientCloseChan` is used by RC to signal to RTS that the client has disconnected.
- `roChan` ("routine output channel") is written to by RT to carry the part of the output of `.Next()` that pertains to this user. It is read from by RTS. 

