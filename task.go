package main

type Status string

const (
	StatusPending Status = "Pending"
	StatusQueued Status = "Queued"
	StatusProcessing Status = "Processing"
	StatusCompleted Status = "Completed"
)

type Result struct {
	Words int
	Runes int
}

type Task struct {
	ID     int
	Input  string
	Status Status
	Result Result
}
