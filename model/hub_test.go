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

func TestHub(t *testing.T) {

	t.Run("Add and get client works on correctly specified clients", func(t *testing.T) {
		tests := []struct {
			publicKey PublicKey
		}{
			{(PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))},
			{(PublicKey)([]byte("\x46\x1d\xe9\xfb\x06\xb0\xa7\xf9\xd3\xe5\x0d\xa3\x0c\x8c\xfa\xc1\xb3\xe6\x11\xec\xa9\x99\xb3\xb3\xc1\x8f\xab\xe3\x96\x18\xf1\x25\x7f\x74\xb5\xf5\xb0\x3f\x7f\x79\xcc\x7e\x5e\x68\x67\xa3\x63\x56\x1d\xc0\x28\xd6\x32\xa1\x45\xb2\xc0\x04\x73\x60\x00\x3a\x73\x17"))},
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
			{(PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))},
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
			{(PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))},
			{(PublicKey)([]byte("\x46\x1d\xe9\xfb\x06\xb0\xa7\xf9\xd3\xe5\x0d\xa3\x0c\x8c\xfa\xc1\xb3\xe6\x11\xec\xa9\x99\xb3\xb3\xc1\x8f\xab\xe3\x96\x18\xf1\x25\x7f\x74\xb5\xf5\xb0\x3f\x7f\x79\xcc\x7e\x5e\x68\x67\xa3\x63\x56\x1d\xc0\x28\xd6\x32\xa1\x45\xb2\xc0\x04\x73\x60\x00\x3a\x73\x17"))},
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
			{(PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))},
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
