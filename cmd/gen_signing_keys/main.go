package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/matrix-org/policyserv/config"
	_ "github.com/matrix-org/policyserv/logging" // always set up logging
	signingkey "github.com/t2bot/go-matrix-signing-key"
)

func main() {
	overwrite := flag.Bool("overwrite", false, "Overwrite existing files if they exist.")
	flag.Parse()

	c, err := config.NewInstanceConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Note: we don't treat generation errors as fatal because we might add a 3rd key eventually, and it'd
	// be nice to be able to just re-run the tool in-place.

	if err := makeKey(c.HomeserverSigningKeyPath, *overwrite); err != nil {
		log.Println(err)
	}
	if err := makeKey(c.HomeserverEventSigningKeyPath, *overwrite); err != nil {
		log.Println(err)
	}

	log.Println("Done!")
}

func makeKey(toFile string, overwriteIfExists bool) error {
	// "does the file exist" checks like this aren't normally safe because the file could be
	// created while we're doing work, but in this case that's pretty unlikely.
	if !overwriteIfExists {
		if _, err := os.Stat(toFile); err == nil {
			return fmt.Errorf("'%s' already exists - not making a new key for it", toFile)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	key, err := signingkey.Generate()
	if err != nil {
		return err
	}

	log.Printf("Key ID for '%s' is '%s'", toFile, key.KeyID())

	b, err := key.Encode(signingkey.FormatMMR) // the actual format we use doesn't matter
	if err != nil {
		return err
	}

	f, err := os.Create(toFile)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b)
	return err
}
