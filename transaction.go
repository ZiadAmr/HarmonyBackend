package main

import (
	"time"

	"golang.org/x/exp/rand"
)

const IDLEN = 16

// instance of a routine
type Transaction struct {
	id     [IDLEN]byte
	fromCl chan string
	toCl   chan string
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

func makeTransaction() Transaction {
	return makeTransactionWithId(newId())
}

func makeTransactionWithId(id [IDLEN]byte) Transaction {

	fromCl := make(chan string)
	toCl := make(chan string)
	return Transaction{
		id:     id,
		fromCl: fromCl,
		toCl:   toCl,
	}
}
