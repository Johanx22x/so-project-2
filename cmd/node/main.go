package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"so-project-2/internal/models"
)

func main() {
	// Definir flags para recibir parámetros desde la línea de comandos
	idFlag := flag.String("id", "", "Node ID (default from NODE_ID environment variable)")
	hostFlag := flag.String("host", "", "Node host and port (default from NODE_HOST environment variable)")
	parentHostFlag := flag.String("parent", "", "Parent host and port (default from PARENT_HOST environment variable)")
	flag.Parse()

	// Usar variables de entorno si no se proporcionaron flags
	nodeID := getEnvOrDefault("NODE_ID", *idFlag)
	nodeHost := getEnvOrDefault("NODE_HOST", *hostFlag)
	parentHost := getEnvOrDefault("PARENT_HOST", *parentHostFlag)

	// Validar que todos los parámetros estén configurados
	if nodeID == "" || nodeHost == "" || parentHost == "" {
		log.Fatal("Missing required parameters: id, host, or parent")
	}

	// Crear el nodo
	node := models.NewNode(nodeID, nodeHost, parentHost)

	// Registrar el nodo con el nodo padre
	if err := node.RegisterWithParent(); err != nil {
		log.Fatalf("Failed to register node with parent: %v", err)
	}

	// Configurar rutas
	http.HandleFunc("/process", node.ProcessTask)

	// Iniciar servidor
	fmt.Printf("Node %s running on %s\n", node.ID, node.Host)
	log.Fatal(http.ListenAndServe(node.Host, nil))
}

// getEnvOrDefault devuelve el valor de una variable de entorno o un valor predeterminado
func getEnvOrDefault(envKey, fallback string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	return fallback
}
