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
	pkToROChan map[PublicKey](chan RoutineOutput)
	// also requires pkToROChanLock
	transactionSocketCount int
	pkToROChanLock         sync.Mutex

	routine Routine

	riChan chan routineInputWrapper
}

type transactionStatus struct {
	done         bool
	timeoutTimer <-chan time.Time
}

// each client interacting with a given transaction has one of these
type transactionSocket struct {
	fromCl          chan string
	clientCloseChan chan struct{}
	roChan          chan RoutineOutput
	id              [IDLEN]byte
	transaction     *transaction
	status          transactionStatus
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
