package communication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func SendTask(url string, task interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	data, _ := json.Marshal(task)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send task, status code: %d", resp.StatusCode)
	}
	return nil
}

func FreeNode(url string, nodeID string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	data := map[string]string{
		"id": nodeID,
	}
	dataBytes, _ := json.Marshal(data)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(dataBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to free node, status code: %d", resp.StatusCode)
	}
	return nil
}
