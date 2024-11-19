package main

import (
	"fmt"
	"log"
	"net/http"
	"so-project-2/internal/models"
)

func main() {
	parent := models.NewDistributedOS()

	http.HandleFunc("/register", parent.RegisterNode)
	http.HandleFunc("/assign", parent.AssignTask)
	http.HandleFunc("/free", parent.FreeNode)

	port := "8080"
	fmt.Printf("Parent node running on port %s\n", port)
	go func() {
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}()

	parent.ProcessTasks()
}
