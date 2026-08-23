package model

// threadsafe

import (
	"errors"
	"sync"
)

type Hub = genericHub[*Client]

// make hub generic for testing purposes
type genericHub[C interface{}] struct {
	AllowedHostnames []string
	clients          map[PublicKey]C
	lock             sync.Mutex
}

func NewHub(allowedHostnames []string) *Hub {
	return newGenericHub[*Client](allowedHostnames)
}

func newGenericHub[C interface{}](allowedHostnames []string) *genericHub[C] {
	return &genericHub[C]{
		clients:          make(map[PublicKey]C),
		AllowedHostnames: allowedHostnames,
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
	defer h.lock.Unlock()
	h.lock.Lock()

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
