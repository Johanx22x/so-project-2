package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"so-project-2/internal/communication"
	"time"
)

type Node struct {
	ID         string
	Host       string
	ParentHost string
	IsBusy     bool
}

func NewNode(id, host, parentHost string) *Node {
	return &Node{
		ID:         id,
		Host:       host,
		ParentHost: parentHost,
		IsBusy:     false,
	}
}

func (n *Node) RegisterWithParent() error {
	url := fmt.Sprintf("http://%s/register", n.ParentHost)
	data := map[string]string{
		"id":   n.ID,
		"host": n.Host,
	}
	return communication.SendTask(url, data)
}

func (n *Node) Free() {
	url := fmt.Sprintf("http://%s/free", n.ParentHost)
	err := communication.FreeNode(url, n.ID)
	if err != nil {
		fmt.Printf("Failed to free node %s: %v\n", n.ID, err)
		return
	}
	fmt.Printf("Node %s freed\n", n.ID)
}

func (n *Node) ProcessTask(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	defer n.Free()
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid task data", http.StatusBadRequest)
		return
	}

	fmt.Printf("Node %s processing Task with duration %d\n", n.ID, task.Duration)
	time.Sleep(time.Duration(task.Duration) * time.Second) // XXX: Simular procesamiento
	fmt.Printf("Node %s completed Task with duration %d\n", n.ID, task.Duration)
	w.WriteHeader(http.StatusOK)
}
