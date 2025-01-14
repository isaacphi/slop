package main

import (
	"log"
	"os"

	"github.com/isaacphi/wheel/internal/config"
	"github.com/isaacphi/wheel/internal/db"
	"github.com/isaacphi/wheel/cmd/wheel/root"
)

func main() {
	// Initialize config
	if err := config.Initialize(); err != nil {
		log.Fatal("Failed to initialize config:", err)
	}

	// Initialize database
	if err := db.Initialize(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Run the command
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}