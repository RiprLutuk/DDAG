// Command auth-service is the DDAG OAuth2 token service.
package main

import (
	"log"

	"github.com/ddag/ddag/internal/authservice"
)

func main() {
	if err := authservice.Run(); err != nil {
		log.Fatalf("auth-service: %v", err)
	}
}
