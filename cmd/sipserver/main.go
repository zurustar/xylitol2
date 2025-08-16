package main

import (
	"flag"
	"log"

	"github.com/zurustar/xylitol2/internal/server"
)

func main() {
	var configFile = flag.String("config", "config.yaml", "Configuration file path")
	flag.Parse()

	// Create server instance
	sipServer := server.NewSIPServer()

	// Load configuration
	if err := sipServer.LoadConfig(*configFile); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Run server with signal handling
	if err := sipServer.RunWithSignalHandling(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}