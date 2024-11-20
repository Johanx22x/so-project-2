package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"so-project-2/internal/models"
	"syscall"
)

func main() {
	portFlag := flag.String("port", "", "Node port")
	flag.Parse()

	nodePort := getEnvOrDefault("NODE_PORT", *portFlag)
	if nodePort == "" {
		log.Fatal("Missing required parameter: port")
	}

	node := models.NewNode("localhost", nodePort)

	// Register routes
	http.HandleFunc("/ping", node.Ping)
	http.HandleFunc("/status", node.GetStatus)
	http.HandleFunc("/peer", node.AddPeer)
	http.HandleFunc("/unpeer", node.RemovePeer)
	http.HandleFunc("/shutdown", node.Shutdown)
	http.HandleFunc("/task", node.AddTask)
	http.HandleFunc("/resources", node.GetResources)
	http.HandleFunc("/resource", node.AddResource)
	http.HandleFunc("/resource/uncheck", node.AddResourceWithoutCheck)
	http.HandleFunc("/unresource", node.RemoveResource)
	http.HandleFunc("/get-resource", node.GetResource)
	http.HandleFunc("/request-resource", node.RequestResource)
	http.HandleFunc("/tasks-for-node", node.GetTasksForNode)

	// Start node server with graceful shutdown
	server := &http.Server{Addr: ":" + node.Port}

	// Goroutine para manejar el servidor HTTP
	go func() {
		fmt.Printf("Node started at %s:%s\n", node.Host, node.Port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Task loop
	go node.TaskLoop()

	// Peer checking loop
	go node.PeerCheckLoop()

	// Signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	sig := <-signalChan
	log.Printf("Received signal: %v. Shutting down gracefully...", sig)
	node.Cleanup()

	// Apagar el servidor HTTP
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Error shutting down server: %v", err)
	}

	log.Println("Server gracefully stopped.")
}

func getEnvOrDefault(envKey, fallback string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	return fallback
}
