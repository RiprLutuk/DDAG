// Command admin-backend is the DDAG control-plane API for the dashboard.
package main

import (
	"log"

	"github.com/ddag/ddag/internal/adminsvc"
)

func main() {
	if err := adminsvc.Run(); err != nil {
		log.Fatalf("admin-backend: %v", err)
	}
}
