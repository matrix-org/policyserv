package main

import (
	"log"
	"os"

	signing_key "github.com/t2bot/go-matrix-signing-key"
)

func decodeSigningKey(filePath string) *signing_key.Key {
	var key *signing_key.Key
	func() { // we use a closure to defer the key close early
		f, keyErr := os.Open(filePath)
		if keyErr != nil {
			log.Fatal(keyErr) // configuration error
		}
		defer f.Close()
		key, keyErr = signing_key.DecodeKey(f)
		if keyErr != nil {
			log.Fatal(keyErr) // likely pointing at the wrong file
		}
	}()
	return key
}
