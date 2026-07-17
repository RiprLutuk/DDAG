// Command policy-engine is a DDAG service.
package main

import (
	"log"

	"github.com/ddag/ddag/internal/policyengine"
)

func main() {
	if err := policyengine.Run(); err != nil {
		log.Fatalf("policy-engine: %v", err)
	}
}
