package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Node struct {
	Host     string
	Port     string
	Tasks    []Task
	IsBusy   bool
	mu       sync.Mutex
	peerList []Peer
}

func NewNode(host, port string) *Node {
	return &Node{
		Host:     host,
		Port:     port,
		Tasks:    []Task{},
		IsBusy:   false,
		peerList: []Peer{},
	}
}

func (n *Node) Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (n *Node) AddPeer(w http.ResponseWriter, r *http.Request) {
	// Get peer data from JSON body
	var peer Peer
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Add peer to the list
	n.mu.Lock()

	// Check if peer already exists
	for _, p := range n.peerList {
		if (p.Host == peer.Host && p.Port == peer.Port) || (peer.Host == n.Host && peer.Port == n.Port) {
			n.mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	// Add peer to the list
	n.peerList = append(n.peerList, peer)
	n.mu.Unlock()
	fmt.Printf("Peer added: %s:%s\n", peer.Host, peer.Port)
	w.WriteHeader(http.StatusOK)

	go func() {
		// Send peer request to new peer in JSON format
		peerJSON, err := json.Marshal(Peer{Host: n.Host, Port: n.Port})
		if err != nil {
			fmt.Println("Failed to marshal peer data")
			return
		}

		_, err = http.Post(fmt.Sprintf("http://%s:%s/peer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			fmt.Printf("Failed to send peer request to %s:%s\n", peer.Host, peer.Port)
		}

		// Tell new peer to peer with all other peers
		n.mu.Lock()
		for _, p := range n.peerList {
			if p.Host != peer.Host || p.Port != peer.Port {
				peerJSON, err := json.Marshal(Peer{Host: p.Host, Port: p.Port})
				if err != nil {
					fmt.Println("Failed to marshal peer data")
					n.mu.Unlock()
					return
				}

				_, err = http.Post(fmt.Sprintf("http://%s:%s/peer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
				if err != nil {
					fmt.Printf("Failed to send peer request to %s:%s\n", peer.Host, peer.Port)
				}
			}
		}
		n.mu.Unlock()
	}()
}

func (n *Node) GetStatus(w http.ResponseWriter, r *http.Request) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Write status to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		IsBusy bool `json:"isBusy"`
		Tasks  int  `json:"tasks"`
	}{
		IsBusy: n.IsBusy,
		Tasks:  len(n.Tasks),
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (n *Node) GetPeerList(w http.ResponseWriter, r *http.Request) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Write peer list to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(n.peerList); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (n *Node) RemovePeer(w http.ResponseWriter, r *http.Request) {
	// Get peer data from JSON body
	var peer Peer
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Remove peer from the list
	n.mu.Lock()
	for i, p := range n.peerList {
		if p.Host == peer.Host && p.Port == peer.Port {
			n.peerList = append(n.peerList[:i], n.peerList[i+1:]...)
			fmt.Printf("Peer removed: %s:%s\n", peer.Host, peer.Port)
			n.mu.Unlock()
			return
		}
	}
	n.mu.Unlock()
	w.WriteHeader(http.StatusNotFound)
}

func (n *Node) Shutdown(w http.ResponseWriter, r *http.Request) {
	// Send unpeer request to all peers
	n.mu.Lock()
	for _, peer := range n.peerList {
		peerJSON, err := json.Marshal(Peer{Host: n.Host, Port: n.Port})
		if err != nil {
			fmt.Println("Failed to marshal peer data")
			n.mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = http.Post(fmt.Sprintf("http://%s:%s/unpeer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			fmt.Printf("Failed to send unpeer request to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()

	// Delegate tasks to peers
	n.mu.Lock()
	for _, task := range n.Tasks {
		// Delegate to first peer
		peer := n.peerList[0]
		peerJSON, err := json.Marshal(task)
		if err != nil {
			fmt.Println("Failed to marshal task data")
			n.mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Printf("Task delegated to %s:%s\n", peer.Host, peer.Port)
		_, err = http.Post(fmt.Sprintf("http://%s:%s/task", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			fmt.Printf("Failed to send task to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	fmt.Println("Node shutting down")
	os.Exit(0)
}

func (n *Node) Cleanup() {
	// Send unpeer request to all peers
	n.mu.Lock()
	for _, peer := range n.peerList {
		peerJSON, err := json.Marshal(Peer{Host: n.Host, Port: n.Port})
		if err != nil {
			fmt.Println("Failed to marshal peer data")
			n.mu.Unlock()
			return
		}

		_, err = http.Post(fmt.Sprintf("http://%s:%s/unpeer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			fmt.Printf("Failed to send unpeer request to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()

	// Delegate tasks to peers
	n.mu.Lock()
	for _, task := range n.Tasks {
		// Delegate to first peer
		peer := n.peerList[0]
		peerJSON, err := json.Marshal(task)
		if err != nil {
			fmt.Println("Failed to marshal task data")
			n.mu.Unlock()
			return
		}

		fmt.Printf("Task delegated to %s:%s\n", peer.Host, peer.Port)
		_, err = http.Post(fmt.Sprintf("http://%s:%s/task", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			fmt.Printf("Failed to send task to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()
}

func (n *Node) AddTask(w http.ResponseWriter, r *http.Request) {
	// Add a task to the node
	n.mu.Lock()
	defer n.mu.Unlock()

	// Get task data from JSON body
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Add task to the list
	n.Tasks = append(n.Tasks, task)
	fmt.Printf("Task added: %d\n", task.Duration)

	w.WriteHeader(http.StatusOK)
}

func (n *Node) TaskLoop() {
	for {
		n.mu.Lock()
		// Check if there are tasks to do
		if len(n.Tasks) == 0 {
			n.mu.Unlock()
			continue
		}

		// Check if node is free
		if !n.IsBusy {
			n.IsBusy = true

			task := n.Tasks[0]
			n.Tasks = n.Tasks[1:]
			n.mu.Unlock()

			go n.performTask(task)
			continue
		}

		// Check if there's a peer to delegate the task to
		if len(n.peerList) == 0 {
			n.mu.Unlock()
			continue
		}

		for _, peer := range n.peerList {
			// Get peer status
			resp, err := http.Get(fmt.Sprintf("http://%s:%s/status", peer.Host, peer.Port))
			if err != nil {
				fmt.Printf("Failed to get peer status from %s:%s\n", peer.Host, peer.Port)
				continue
			}

			// Decode peer status
			var status struct {
				IsBusy bool `json:"isBusy"`
				Tasks  int  `json:"tasks"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				fmt.Printf("Failed to decode peer status from %s:%s\n", peer.Host, peer.Port)
				continue
			}

			// Check if peer is free
			if status.IsBusy {
				continue
			}

			// Delegate task to peer
			peerJSON, err := json.Marshal(n.Tasks[0])
			if err != nil {
				fmt.Println("Failed to marshal task data")
				continue
			}

			_, err = http.Post(fmt.Sprintf("http://%s:%s/task", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
			if err != nil {
				fmt.Printf("Failed to send task to %s:%s\n", peer.Host, peer.Port)
				continue
			}

			// Remove task from the list
			fmt.Printf("Task delegated to %s:%s\n", peer.Host, peer.Port)
			n.Tasks = n.Tasks[1:]
			break
		}

		n.mu.Unlock()
	}
}

func (n *Node) performTask(task Task) {
	fmt.Printf("Performing task for %d seconds\n", task.Duration)
	// Simulate task execution
	time.Sleep(time.Duration(task.Duration) * time.Second)
	n.IsBusy = false
	fmt.Printf("Task completed\n")
}

func (n *Node) PeerCheckLoop() {
	for {
		time.Sleep(10 * time.Second) // Check every 10 seconds
		n.mu.Lock()
		// Check if there are peers to check
		if len(n.peerList) == 0 {
			n.mu.Unlock()
			continue
		}

		for _, peer := range n.peerList {
			// ping peer
			resp, err := http.Get(fmt.Sprintf("http://%s:%s/ping", peer.Host, peer.Port))
			if err != nil || resp.StatusCode != http.StatusOK {
				log.Printf("Peer %s:%s has gone offline unexpectedly, removing from list...\n", peer.Host, peer.Port)
				// Remove peer from list
				for i, p := range n.peerList {
					if p.Host == peer.Host && p.Port == peer.Port {
						n.peerList = append(n.peerList[:i], n.peerList[i+1:]...)
						fmt.Printf("Peer removed: %s:%s\n", peer.Host, peer.Port)
						break
					}
				}
			}
		}
		n.mu.Unlock()
	}
}
