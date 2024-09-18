package model

import "errors"

// use this interface in place of the Client struct
// there may be other stuff in the Client struct that we don't care about here.
// use interface cos it's easier to mock if needed
type HasPublicKey interface {
	GetPublicKey() *PublicKey
}

type Hub = GenericHub[*Client]

type GenericHub[C HasPublicKey] struct {
	clients map[PublicKey]C
}

func NewHub() *Hub {
	return newGenericHub[*Client]()
}

func newGenericHub[C HasPublicKey]() *GenericHub[C] {
	return &GenericHub[C]{
		clients: make(map[PublicKey]C),
	}
}

func (h GenericHub[C]) AddClient(client C) error {

	publicKey := client.GetPublicKey()

	if publicKey == nil {
		return errors.New("client has nil public key")
	}

	_, alreadyExists := h.clients[*publicKey]
	if alreadyExists {
		return errors.New("client with public key already exists")
	}

	h.clients[*publicKey] = client
	return nil
}

func (h GenericHub[C]) GetClient(key PublicKey) (C, bool) {
	cl, exists := h.clients[key]
	return cl, exists
}

func (h GenericHub[C]) DeleteClient(key PublicKey) error {
	_, exists := h.clients[key]
	if !exists {
		return errors.New("client with public key does not exist")
	}
	delete(h.clients, key)
	return nil
}
