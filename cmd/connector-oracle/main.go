// Command connector-oracle is the DDAG oracle connector service. It shares the
// connector implementation but deploys as its own pod/image (PRD §19.1).
package main

import (
	"log"

	"github.com/ddag/ddag/internal/connector"
)

func main() {
	if err := connector.Run("oracle"); err != nil {
		log.Fatalf("connector-oracle: %v", err)
	}
}
