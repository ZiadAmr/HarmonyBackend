package model

import (
	"strconv"
	"testing"
)

// mock client type
type ClientMockForHub struct {
	publicKey *PublicKey
}

func (c *ClientMockForHub) GetPublicKey() *PublicKey {
	return c.publicKey
}

var pk0 = (PublicKey)("MCowBQYDK2VwAyEAUFRxKDllkUY843/zVOPE67zGqkGoMZd7dGKl2+9+pYQ=")

// var privateKey0 = "MC4CAQAwBQYDK2VwBCIEILLK2qyMQi162qzsJ2pV5bS5tX/6XEgWtw62eUKOKLAF"

var pk1 = (PublicKey)("MCowBQYDK2VwAyEA1x5dCGTiFyoAGPP8XTzv58tZQHx5RB5E+5xFX5xwMFQ=")

func TestHub(t *testing.T) {

	t.Run("Add and get client works on correctly specified clients", func(t *testing.T) {
		tests := []struct {
			publicKey PublicKey
		}{
			{pk0},
			{pk1},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				// mocks
				hub := newGenericHub[*ClientMockForHub]()
				client := &ClientMockForHub{publicKey: &tt.publicKey}

				// add client to hub and attempt to get them back
				err := hub.AddClient(tt.publicKey, client)
				if err != nil {
					t.Errorf(err.Error())
				}

				returnedClient, exists := hub.GetClient(tt.publicKey)

				if returnedClient != client {
					t.Errorf("Expected %v got %v", client, returnedClient)
				}

				if !exists {
					t.Errorf("Client exists: expected %t got %t", true, exists)
				}

			})
		}
	})

	t.Run("adding 2 clients with the same public key fails", func(t *testing.T) {
		tests := []struct {
			publicKey PublicKey
		}{
			{pk0},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {

				hub := newGenericHub[*ClientMockForHub]()

				// add first client directly
				client0 := &ClientMockForHub{publicKey: &tt.publicKey}
				hub.clients[tt.publicKey] = client0

				// use proper method to add second client
				client1 := &ClientMockForHub{publicKey: &tt.publicKey}
				err1 := hub.AddClient(tt.publicKey, client1)

				if err1 == nil {
					t.Errorf("Expected adding the client to fail")
				}

			})
		}
	})

	// t.Run("Adding client with nil public key fails", func(t *testing.T) {

	// 	hub := NewHub()
	// 	client := &Client{publicKey: nil}
	// 	err := hub.AddClient(nil, client)

	// 	if err == nil {
	// 		t.Errorf("Expected an error")
	// 	}

	// })

	t.Run("Deleting clients if they exist succeeds", func(t *testing.T) {
		tests := []struct {
			publicKey PublicKey
		}{
			{pk0},
			{pk1},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				hub := newGenericHub[*ClientMockForHub]()
				client := &ClientMockForHub{publicKey: &tt.publicKey}

				// add client directly
				hub.clients[tt.publicKey] = client

				err := hub.DeleteClient(tt.publicKey)

				if err != nil {
					t.Errorf(err.Error())
				}

			})
		}
	})

	t.Run("Deleting clients if they do not exist fails", func(t *testing.T) {
		tests := []struct {
			publicKey PublicKey
		}{
			{pk0},
		}

		for i, tt := range tests {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				hub := NewHub()

				err := hub.DeleteClient(tt.publicKey)

				if err == nil {
					t.Errorf("Expected an error")
				}

			})
		}
	})
}
