package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"so-project-2/internal/communication"
	"sync"
)

type DistributedOS struct {
	Nodes []*Node
	Tasks []Task
	mu    sync.Mutex
}

func NewDistributedOS() *DistributedOS {
	return &DistributedOS{
		Nodes: []*Node{},
		Tasks: []Task{},
	}
}

func (dos *DistributedOS) RegisterNode(w http.ResponseWriter, r *http.Request) {
	var node Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "Invalid node data", http.StatusBadRequest)
		return
	}

	dos.mu.Lock()
	defer dos.mu.Unlock()
	dos.Nodes = append(dos.Nodes, &node)
	fmt.Printf("Node %s registered at %s\n", node.ID, node.Host)
	w.WriteHeader(http.StatusOK)
}

func (dos *DistributedOS) AssignTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Invalid task data", http.StatusBadRequest)
		return
	}

	dos.mu.Lock()
	defer dos.mu.Unlock()
	dos.Tasks = append(dos.Tasks, task)
	fmt.Printf("Task with duration %d queued\n", task.Duration)
	w.WriteHeader(http.StatusOK)
}

func (dos *DistributedOS) FreeNode(w http.ResponseWriter, r *http.Request) {
	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid node data", http.StatusBadRequest)
		return
	}

	dos.mu.Lock()
	defer dos.mu.Unlock()
	for _, node := range dos.Nodes {
		if node.ID == data["id"] {
			node.IsBusy = false
			fmt.Printf("Node %s freed\n", node.ID)
			break
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (dos *DistributedOS) ProcessTasks() {
	for {
		dos.mu.Lock()
		if len(dos.Tasks) == 0 || len(dos.Nodes) == 0 {
			dos.mu.Unlock()
			continue
		}

		task := dos.Tasks[0]
		dos.mu.Unlock()

		dos.mu.Lock()
		locked := true
		for _, node := range dos.Nodes {
			if !node.IsBusy {
				node.IsBusy = true
				dos.mu.Unlock()
				locked = false
				go func(n *Node, t Task) {
					fmt.Printf("Assigning task with duration %d to node %s\n", t.Duration, n.ID)
					url := fmt.Sprintf("http://%s/process", n.Host)
					if err := communication.SendTask(url, t); err != nil {
						fmt.Printf("Failed to send task to node %s: %v\n", n.ID, err)
					}
				}(node, task)
				break
			}
		}

		if locked {
			dos.mu.Unlock()
		} else {
			dos.Tasks = dos.Tasks[1:]
		}
	}
}
