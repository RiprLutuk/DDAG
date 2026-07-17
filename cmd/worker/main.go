// Command worker is a DDAG service.
package main

import (
	"log"

	"github.com/ddag/ddag/internal/workersvc"
)

func main() {
	if err := workersvc.Run(); err != nil {
		log.Fatalf("worker: %v", err)
	}
}
