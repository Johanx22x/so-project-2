package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"so-project-2/internal/utils"
	"sync"
	"time"
)

type Node struct {
	Host      string
	Port      string
	Tasks     []Task
	IsBusy    bool
	mu        sync.Mutex
	peers     []Peer
	Resources []Resource
}

func NewNode(host, port string) *Node {
	return &Node{
		Host:      host,
		Port:      port,
		Tasks:     []Task{},
		IsBusy:    false,
		peers:     []Peer{},
		Resources: []Resource{},
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
	for _, p := range n.peers {
		if (p.Host == peer.Host && p.Port == peer.Port) || (peer.Host == n.Host && peer.Port == n.Port) {
			n.mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	// Add peer to the list
	n.peers = append(n.peers, peer)
	n.mu.Unlock()
	log.Printf("["+utils.Colorize("33", "COMM")+"] Peer added: %s:%s\n", peer.Host, peer.Port)
	w.WriteHeader(http.StatusOK)

	go func() {
		// Send peer request to new peer in JSON format
		peerJSON, err := json.Marshal(Peer{Host: n.Host, Port: n.Port})
		if err != nil {
			log.Printf("[" + utils.Colorize("31", "ERROR") + "] Failed to marshal peer data\n")
			return
		}

		_, err = http.Post(fmt.Sprintf("http://%s:%s/peer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send peer request to %s:%s\n", peer.Host, peer.Port)
		}

		// Tell new peer to peer with all other peers
		n.mu.Lock()
		for _, p := range n.peers {
			if p.Host != peer.Host || p.Port != peer.Port {
				peerJSON, err := json.Marshal(Peer{Host: p.Host, Port: p.Port})
				if err != nil {
					log.Printf("[" + utils.Colorize("31", "ERROR") + "] Failed to marshal peer data\n")
					n.mu.Unlock()
					return
				}

				_, err = http.Post(fmt.Sprintf("http://%s:%s/peer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
				if err != nil {
					log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send peer request to %s:%s\n", peer.Host, peer.Port)
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
	if err := json.NewEncoder(w).Encode(n.peers); err != nil {
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
	for i, p := range n.peers {
		if p.Host == peer.Host && p.Port == peer.Port {
			n.peers = append(n.peers[:i], n.peers[i+1:]...)
			log.Printf("["+utils.Colorize("33", "COMM")+"] Peer removed: %s:%s\n", peer.Host, peer.Port)
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
	for _, peer := range n.peers {
		peerJSON, err := json.Marshal(Peer{Host: n.Host, Port: n.Port})
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send unpeer request to %s:%s\n", peer.Host, peer.Port)
			n.mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = http.Post(fmt.Sprintf("http://%s:%s/unpeer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send unpeer request to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()

	// Delegate tasks to peers
	n.mu.Lock()
	for _, task := range n.Tasks {
		// Delegate to first peer
		peer := n.peers[0]
		peerJSON, err := json.Marshal(task)
		if err != nil {
			log.Printf("[" + utils.Colorize("31", "ERROR") + "] Failed to marshal task data\n")
			n.mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("["+utils.Colorize("33", "COMM")+"] Task delegated to %s:%s\n", peer.Host, peer.Port)
		_, err = http.Post(fmt.Sprintf("http://%s:%s/task", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to delegate task to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	log.Println("Node shutting down")
	os.Exit(0)
}

func (n *Node) Cleanup() {
	// Send unpeer request to all peers
	n.mu.Lock()
	for _, peer := range n.peers {
		peerJSON, err := json.Marshal(Peer{Host: n.Host, Port: n.Port})
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send unpeer request to %s:%s\n", peer.Host, peer.Port)
			n.mu.Unlock()
			return
		}

		_, err = http.Post(fmt.Sprintf("http://%s:%s/unpeer", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send unpeer request to %s:%s\n", peer.Host, peer.Port)
		}
	}
	n.mu.Unlock()

	// Delegate tasks to peers
	n.mu.Lock()
	for _, task := range n.Tasks {
		// Delegate to first peer
		peer := n.peers[0]
		peerJSON, err := json.Marshal(task)
		if err != nil {
			log.Printf("[" + utils.Colorize("31", "ERROR") + "] Failed to marshal task data\n")
			n.mu.Unlock()
			return
		}

		log.Printf("["+utils.Colorize("33", "COMM")+"] Task delegated to %s:%s\n", peer.Host, peer.Port)
		_, err = http.Post(fmt.Sprintf("http://%s:%s/task", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to delegate task to %s:%s\n", peer.Host, peer.Port)
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
	log.Printf("[" + utils.Colorize("32", "INFO") + "] Task added\n")

	w.WriteHeader(http.StatusOK)
}

func (n *Node) TaskLoop() {
	for {
		n.mu.Lock()

		// Check if there are tasks to do
		if len(n.Tasks) == 0 {
			n.mu.Unlock()
			time.Sleep(100 * time.Millisecond) // Avoid busy waiting
			continue
		}

		// Check if node is free
		if !n.IsBusy {
			n.IsBusy = true

			task := n.Tasks[0]
			n.Tasks = n.Tasks[1:] // Remove the task from the queue
			n.mu.Unlock()

			go n.performTask(task) // Perform the task locally
			continue
		}

		// Attempt to delegate the task to the least loaded peer
		task := n.Tasks[0]
		if n.GetLeastLoadedPeer(task) {
			// Remove task from queue if delegation was successful
			n.Tasks = n.Tasks[1:]
		}

		n.mu.Unlock()
		time.Sleep(100 * time.Millisecond) // Avoid busy waiting
	}
}

func (n *Node) performTask(task Task) {
	log.Printf("["+utils.Colorize("32", "INFO")+"] Performing task for %d seconds\n", task.Duration)

	var resourcesFlag map[string]bool = make(map[string]bool)
	for _, path := range task.Resources {
		resourcesFlag[path] = false
	}

	// Mark resources as in use
	n.mu.Lock()
	for _, path := range task.Resources {
		// Check if resource exists in this node
		found := false
		for _, r := range n.Resources {
			if r.Path == path {
				// Check if resource is in use, if not wait until it's free
				for r.InUse {
					time.Sleep(1 * time.Millisecond)
				}
				found = true
				break
			}
		}

		if found {
			resourcesFlag[path] = true
		} else {
			for {
				// Check if resource exists in other nodes
				for _, peer := range n.peers {
					n.mu.Unlock()
					resp, err := http.Get(fmt.Sprintf("http://%s:%s/resources", peer.Host, peer.Port))
					if err != nil {
						log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to get resources from %s:%s\n", peer.Host, peer.Port)
						continue
					}
					n.mu.Lock()

					var resources []Resource
					if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
						log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to decode resources from %s:%s\n", peer.Host, peer.Port)
						continue
					}

					for _, r := range resources {
						if r.Path == path {
							// Try to get resource
							n.mu.Unlock()
							res, err := http.Post(fmt.Sprintf("http://%s:%s/get-resource", peer.Host, peer.Port), "application/json", bytes.NewBuffer(
								[]byte(fmt.Sprintf(`{"host":"%s","port":"%s","path":"%s"}`, n.Host, n.Port, path))))
							if err != nil {
								log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to get resource %s from %s:%s\n", path, peer.Host, peer.Port)
							}
							if res.StatusCode == http.StatusOK {
								resourcesFlag[path] = true
								log.Printf("["+utils.Colorize("33", "COMM")+"] Resource %s found in %s:%s\n", path, peer.Host, peer.Port)
							}
							found = true
							n.mu.Lock()
							break
						}
					}
				}

				if !found {
					log.Printf("["+utils.Colorize("31", "ERROR")+"] Resource %s not found in any node, cannot perform task\n", path)
					n.IsBusy = false
					n.mu.Unlock()
					return
				}

				if found && resourcesFlag[path] {
					break
				}

				found = false
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	n.mu.Unlock()

	// Check if all resources were obtained
	for _, obtained := range resourcesFlag {
		if !obtained {
			log.Printf("[" + utils.Colorize("31", "INFO") + "] Failed to get all resources, cannot perform task\n")
			n.mu.Lock()
			n.IsBusy = false
			n.mu.Unlock()
			return
		}
	}

	// Mark resources as in use
	n.mu.Lock()
	for _, path := range task.Resources {
		for j, r := range n.Resources {
			if r.Path == path {
				log.Printf("["+utils.Colorize("32", "INFO")+"] Resource %s is in use by task\n", path)
				n.Resources[j].InUse = true
				break
			}
		}
	}
	n.mu.Unlock()

	// Simulate task execution
	time.Sleep(time.Duration(task.Duration) * time.Second)

	// Mark resources as free
	n.mu.Lock()
	for _, path := range task.Resources {
		for j, r := range n.Resources {
			if r.Path == path {
				n.Resources[j].InUse = false
				break
			}
		}
	}
	n.IsBusy = false
	n.mu.Unlock()
	log.Printf("[" + utils.Colorize("32", "INFO") + "] Task completed\n")
}

func (n *Node) PeerCheckLoop() {
	for {
		time.Sleep(10 * time.Second) // Check every 10 seconds
		n.mu.Lock()
		// Check if there are peers to check
		if len(n.peers) == 0 {
			n.mu.Unlock()
			continue
		}

		for _, peer := range n.peers {
			// ping peer
			resp, err := http.Get(fmt.Sprintf("http://%s:%s/ping", peer.Host, peer.Port))
			if err != nil || resp.StatusCode != http.StatusOK {
				log.Printf("["+utils.Colorize("33", "COMM")+"] Peer %s:%s has gone offline unexpectedly, removing from peers\n", peer.Host, peer.Port)
				n.RedistributeTasks(peer)
				// Remove peer from list
				for i, p := range n.peers {
					if p.Host == peer.Host && p.Port == peer.Port {
						n.peers = append(n.peers[:i], n.peers[i+1:]...)
						log.Printf("["+utils.Colorize("33", "COMM")+"] Peer removed: %s:%s\n", peer.Host, peer.Port)
						break
					}
				}
			}
		}
		n.mu.Unlock()
	}
}

func (n *Node) GetResources(w http.ResponseWriter, r *http.Request) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Write resources to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(n.Resources); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (n *Node) AddResource(w http.ResponseWriter, r *http.Request) {
	// Get resource data from JSON body
	var resource Resource
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check if resource already exists in the list
	n.mu.Lock()
	for _, r := range n.Resources {
		if r.Path == resource.Path {
			n.mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			return
		}
	}
	n.mu.Unlock()

	// or in other nodes
	n.mu.Lock()
	for _, peer := range n.peers {
		resp, err := http.Get(fmt.Sprintf("http://%s:%s/resources", peer.Host, peer.Port))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to get resources from %s:%s\n", peer.Host, peer.Port)
			continue
		}

		var resources []Resource
		if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to decode resources from %s:%s\n", peer.Host, peer.Port)
			continue
		}

		for _, r := range resources {
			if r.Path == resource.Path {
				w.WriteHeader(http.StatusConflict)
				n.mu.Unlock()
				return
			}
		}
	}
	n.mu.Unlock()

	// Add resource to the list
	n.mu.Lock()
	n.Resources = append(n.Resources, resource)
	log.Printf("["+utils.Colorize("32", "INFO")+"] Resource %s added\n", resource.Path)
	n.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (n *Node) AddResourceWithoutCheck(w http.ResponseWriter, r *http.Request) {
	// Get resource data from JSON body
	var resource Resource
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Add resource to the list
	n.mu.Lock()
	n.Resources = append(n.Resources, resource)
	log.Printf("["+utils.Colorize("32", "INFO")+"] Resource %s added\n", resource.Path)
	n.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (n *Node) RemoveResource(w http.ResponseWriter, r *http.Request) {
	// Get resource data from JSON body
	var resource Resource
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Remove resource from the list
	n.mu.Lock()
	for i, r := range n.Resources {
		if r.Path == resource.Path {
			n.Resources = append(n.Resources[:i], n.Resources[i+1:]...)
			log.Printf("["+utils.Colorize("32", "INFO")+"] Resource %s removed\n", resource.Path)
			n.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	n.mu.Unlock()
	w.WriteHeader(http.StatusNotFound)
}

func (n *Node) GetResource(w http.ResponseWriter, r *http.Request) {
	type ResourceRequest struct {
		Host string `json:"host"`
		Port string `json:"port"`
		Path string `json:"path"`
	}

	// Get resource data from JSON body
	var resourceRequest ResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&resourceRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n.mu.Lock()
	// Check if resource exists in the list
	for _, r := range n.Resources {
		if r.Path == resourceRequest.Path {

			if r.InUse {
				break
			}

			// Try to sent resource to peer
			r.Owner = resourceRequest.Host + ":" + resourceRequest.Port
			resourceJSON, err := json.Marshal(r)
			if err != nil {
				fmt.Println("Failed to marshal resource data")
				n.mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Remove resource from the list
			for i, r := range n.Resources {
				if r.Path == resourceRequest.Path {
					n.Resources = append(n.Resources[:i], n.Resources[i+1:]...)
					log.Printf("["+utils.Colorize("32", "INFO")+"] Resource %s sent to %s:%s\n", resourceRequest.Path, resourceRequest.Host, resourceRequest.Port)
				}
			}

			res, err := http.Post(fmt.Sprintf("http://%s:%s/resource/uncheck", resourceRequest.Host, resourceRequest.Port), "application/json", bytes.NewBuffer(resourceJSON))
			if err != nil || res.StatusCode != http.StatusOK {
				log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to send resource to %s:%s\n", resourceRequest.Host, resourceRequest.Port)
				r.Owner = n.Host + ":" + n.Port      // Fallback to original owner
				n.Resources = append(n.Resources, r) // Add resource back to the list
				n.mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			n.mu.Unlock()

			log.Printf("["+utils.Colorize("33", "COMM")+"] Resource %s sent to %s:%s\n", resourceRequest.Path, resourceRequest.Host, resourceRequest.Port)
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	n.mu.Unlock()
	w.WriteHeader(http.StatusNotFound)
}

func (n *Node) RedistributeTasks(failedNode Peer) {
	log.Printf("["+utils.Colorize("33", "COMM")+"] Attempting to redistribute tasks from failed node: %s:%s\n", failedNode.Host, failedNode.Port)

	tasksToRedistribute := n.GetTasksFromPeers(failedNode)
	if len(tasksToRedistribute) == 0 {
		log.Printf("["+utils.Colorize("31", "ERROR")+"] No tasks found for failed node %s:%s\n", failedNode.Host, failedNode.Port)
		return
	}

	log.Printf("["+utils.Colorize("33", "COMM")+"] Redistributing %d tasks from failed node: %s:%s\n", len(tasksToRedistribute), failedNode.Host, failedNode.Port)

	for _, task := range tasksToRedistribute {
		go func(task Task) {
			n.mu.Lock()
			for _, peer := range n.peers {
				peerJSON, err := json.Marshal(task)
				if err != nil {
					log.Printf("[" + utils.Colorize("31", "ERROR") + "] Failed to marshal task data for redistribution\n")
					continue
				}

				resp, err := http.Post(fmt.Sprintf("http://%s:%s/task", peer.Host, peer.Port), "application/json", bytes.NewBuffer(peerJSON))
				if err == nil && resp.StatusCode == http.StatusOK {
					log.Printf("["+utils.Colorize("33", "COMM")+"] Task redistributed to peer %s:%s\n", peer.Host, peer.Port)
					n.mu.Unlock()
					return
				}
			}
			log.Printf("[" + utils.Colorize("31", "ERROR") + "] No available peers to redistribute task\n")
			n.mu.Unlock()
		}(task)
	}
}

func (n *Node) RequestResource(w http.ResponseWriter, r *http.Request) {
	type ResourceRequest struct {
		Path string `json:"path"`
	}

	var req ResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if the resource exists locally
	for _, resource := range n.Resources {
		if resource.Path == req.Path && !resource.InUse {
			log.Printf("["+utils.Colorize("32", "INFO")+"] Resource %s locked for use\n", req.Path)
			for i := range n.Resources {
				if n.Resources[i].Path == req.Path {
					n.Resources[i].InUse = true
				}
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Resource not found locally
	w.WriteHeader(http.StatusNotFound)
}

func (n *Node) GetLeastLoadedPeer(task Task) bool {
	var leastLoadedPeer *Peer
	var minTasks = int(^uint(0) >> 1) // Max int value

	for _, peer := range n.peers {
		resp, err := http.Get(fmt.Sprintf("http://%s:%s/status", peer.Host, peer.Port))
		if err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to get status from peer %s:%s\n", peer.Host, peer.Port)
			continue
		}

		var status struct {
			IsBusy bool `json:"isBusy"`
			Tasks  int  `json:"tasks"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to decode status from peer %s:%s\n", peer.Host, peer.Port)
			continue
		}

		// Select the peer with the least tasks
		if !status.IsBusy && status.Tasks < minTasks {
			minTasks = status.Tasks
			leastLoadedPeer = &peer
		}
	}

	// If a suitable peer is found, delegate the task
	if leastLoadedPeer != nil {
		taskJSON, err := json.Marshal(task)
		if err != nil {
			log.Printf("[" + utils.Colorize("31", "ERROR") + "] Failed to marshal task data\n")
			return false
		}

		resp, err := http.Post(fmt.Sprintf("http://%s:%s/task", leastLoadedPeer.Host, leastLoadedPeer.Port), "application/json", bytes.NewBuffer(taskJSON))
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to delegate task to %s:%s\n", leastLoadedPeer.Host, leastLoadedPeer.Port)
			return false
		}

		log.Printf("["+utils.Colorize("33", "COMM")+"] Task delegated to %s:%s\n", leastLoadedPeer.Host, leastLoadedPeer.Port)
		return true
	}

	// No suitable peer found
	return false
}

func (n *Node) GetTasksForNode(w http.ResponseWriter, r *http.Request) {
	type Request struct {
		Host string `json:"host"`
		Port string `json:"port"`
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// return tasks if the request is for this node
	if req.Host == n.Host && req.Port == n.Port {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(n.Tasks); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// if the request is for another node, check if the node is a peer
	w.WriteHeader(http.StatusNotFound)
}

func (n *Node) GetTasksFromPeers(failedNode Peer) []Task {
	var tasks []Task

	for _, peer := range n.peers {
		reqBody, _ := json.Marshal(failedNode)
		resp, err := http.Post(fmt.Sprintf("http://%s:%s/tasks-for-node", peer.Host, peer.Port), "application/json", bytes.NewBuffer(reqBody))
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to get tasks for node %s:%s from peer %s:%s\n", failedNode.Host, failedNode.Port, peer.Host, peer.Port)
			continue
		}

		var peerTasks []Task
		if err := json.NewDecoder(resp.Body).Decode(&peerTasks); err != nil {
			log.Printf("["+utils.Colorize("31", "ERROR")+"] Failed to decode tasks from peer %s:%s\n", peer.Host, peer.Port)
			continue
		}

		tasks = append(tasks, peerTasks...)
	}

	return tasks
}
