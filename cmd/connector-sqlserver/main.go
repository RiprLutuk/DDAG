// Command connector-sqlserver is the DDAG sqlserver connector service. It shares the
// connector implementation but deploys as its own pod/image (PRD §19.1).
package main

import (
	"log"

	"github.com/ddag/ddag/internal/connector"
)

func main() {
	if err := connector.Run("sqlserver"); err != nil {
		log.Fatalf("connector-sqlserver: %v", err)
	}
}
