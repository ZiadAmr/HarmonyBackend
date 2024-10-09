package model

// TODO NEED TO MAKE THIS THREADSAFE

import (
	"errors"
	"sync"
)

// use this interface in place of the Client struct
// there may be other stuff in the Client struct that we don't care about here.
// use interface cos it's easier to mock if needed
type hasPublicKey interface {
	GetPublicKey() *PublicKey
}

type Hub = genericHub[*Client]

type genericHub[C hasPublicKey] struct {
	clients map[PublicKey]C
	lock    sync.Mutex
}

func NewHub() *Hub {
	return newGenericHub[*Client]()
}

func newGenericHub[C hasPublicKey]() *genericHub[C] {
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
