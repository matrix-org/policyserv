package logging

import (
	"log"
	"os"

	"github.com/matrix-org/policyserv/version"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetPrefix("[policyserv] ")
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	log.Println("Version:", version.Revision)
}
