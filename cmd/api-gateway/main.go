// Command api-gateway is the DDAG dynamic API data plane.
package main

import (
	"log"

	"github.com/ddag/ddag/internal/gatewaysvc"
)

func main() {
	if err := gatewaysvc.Run(); err != nil {
		log.Fatalf("api-gateway: %v", err)
	}
}
