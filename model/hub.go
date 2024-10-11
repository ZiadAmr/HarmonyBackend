package model

// threadsafe

import (
	"errors"
	"sync"
)

type Hub = genericHub[*Client]

// make hub generic for testing purposes
type genericHub[C interface{}] struct {
	clients map[PublicKey]C
	lock    sync.Mutex
}

func NewHub() *Hub {
	return newGenericHub[*Client]()
}

func newGenericHub[C interface{}]() *genericHub[C] {
	return &genericHub[C]{
		clients: make(map[PublicKey]C),
	}
}

func (h *genericHub[C]) AddClient(pk PublicKey, client C) error {
	defer h.lock.Unlock()
	h.lock.Lock()

	_, alreadyExists := h.clients[pk]
	if alreadyExists {
		return errors.New("client with public key already exists")
	}

	h.clients[pk] = client
	return nil
}

func (h *genericHub[C]) GetClient(key PublicKey) (C, bool) {
	cl, exists := h.clients[key]
	return cl, exists
}

func (h *genericHub[C]) DeleteClient(key PublicKey) error {
	defer h.lock.Unlock()
	h.lock.Lock()

	_, exists := h.clients[key]
	if !exists {
		return errors.New("client with public key does not exist")
	}
	delete(h.clients, key)
	return nil
}
