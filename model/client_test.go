package model

import "testing"

func TestSetPublicKey(t *testing.T) {

	pk := (*PublicKey)([]byte("\xcf\xfd\x10\xba\xbe\xd1\x18\x2e\x7d\x8e\x6c\xff\x84\x57\x67\xee\xae\x45\x08\xaa\x13\xcd\x00\x37\x92\x33\xf5\x7f\x79\x9d\xc1\x8c\x1e\xef\xd3\x5b\x51\xdb\x36\xe3\xda\x47\x70\x73\x7a\x3f\x8f\xe7\x5e\xda\x0c\xd3\xc4\x8f\x23\xea\x70\x5f\x32\x34\xb0\x92\x9f\x9e"))

	t.Run("Can be used to set the public key", func(t *testing.T) {

		client := &Client{publicKey: nil}
		err := client.SetPublicKey(pk)

		if err != nil {
			t.Errorf(err.Error())
		}
		if client.publicKey != pk {
			t.Errorf("Expected %v got %v", pk, client.publicKey)
		}
	})

	t.Run("Rejects public keys if already set", func(t *testing.T) {
		client := &Client{publicKey: pk}
		err := client.SetPublicKey(pk)

		if err == nil {
			t.Errorf("Expected an error")
		}

	})
}
