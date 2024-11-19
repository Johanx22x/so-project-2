package models

type Task struct {
	Duration  int      `json:"duration"`
	Resources []string `json:"resources"`
}
