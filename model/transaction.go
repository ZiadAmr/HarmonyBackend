package model

import (
	"time"

	"golang.org/x/exp/rand"
)

const IDLEN = 16

// instance of a routine
type Transaction struct {
	Id      [IDLEN]byte
	Routine Routine
	fromCl  chan string
}

// genreate a random transaction id
func NewId() [IDLEN]byte {
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
