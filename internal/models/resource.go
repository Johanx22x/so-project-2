package models

type Resource struct {
	Path  string `json:"path"`
	Owner string `json:"owner"`
	InUse bool   `json:"in_use"`
}
