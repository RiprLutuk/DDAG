// Command cache-service is a DDAG service.
package main

import (
	"log"

	"github.com/ddag/ddag/internal/cacheservice"
)

func main() {
	if err := cacheservice.Run(); err != nil {
		log.Fatalf("cache-service: %v", err)
	}
}
