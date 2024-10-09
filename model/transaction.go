package model

import (
	"sync"
	"time"

	"golang.org/x/exp/rand"
)

const IDLEN = 16

// instance of a routine
type transaction struct {
	// routine output channels - for communication between users
	pkToROChan     map[PublicKey](chan RoutineOutput)
	pkToROChanLock sync.Mutex

	// prevent concurrent calls to routine.Next() and .Timeout(), and wrapper code in the client
	routineLock sync.Mutex
	routine     Routine
}

type transactionStatus struct {
	done         bool
	timeoutTimer <-chan time.Time
}

type clientTransactionWrapper struct {
	fromCl      chan string
	roChan      chan RoutineOutput
	id          [IDLEN]byte
	transaction *transaction
	status      transactionStatus
}

// genreate a random transaction id
func newId() [IDLEN]byte {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	var seededRand *rand.Rand = rand.New(rand.NewSource(uint64(time.Now().UnixNano())))

	var id [IDLEN]byte
	for i := range id {
		id[i] = charset[seededRand.Intn(len(charset))]
	}
	return id

}

// func MakeTransaction() Transaction {
// 	return MakeTransactionWithId(NewId())
// }

// func MakeTransactionWithId(id [IDLEN]byte) Transaction {

// 	return Transaction{
// 		Id:     id,
// 		fromCl: make(chan string, 1),
// 	}
// }
