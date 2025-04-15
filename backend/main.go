package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	port := flag.Int("port", 8080, "Port to serve the application")
	flag.Parse()

	// Set up the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: setupRoutes(),
	}

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on port %d...\n", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	
	// Wait for an interrupt signal
	<-quit
	log.Println("Shutting down server...")

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt to gracefully shut down the server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}