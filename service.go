package main

import (
	"errors"
	"strings"
	"sync"
)

var (
	ErrEmptyInput   = errors.New("text must not be empty")
	ErrTaskNotFound = errors.New("task not found")
	ErrInvalidState = errors.New("task is not in a valid state for this operation")
)


type TaskService struct {
	mu     sync.RWMutex
	tasks  map[int]*Task
	nextID int

	processor Processor
}

func NewTaskService(processor Processor) *TaskService {
	return &TaskService{
		tasks:     make(map[int]*Task),
		nextID:    1,
		processor: processor,
	}
}

func (s *TaskService) Create(text string) (Task, error) {
	if strings.TrimSpace(text) == "" {
		return Task{}, ErrEmptyInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task := &Task{
		ID:     s.nextID,
		Input:  text,
		Status: StatusPending,
	}

	s.tasks[task.ID] = task
	s.nextID++

	return *task, nil
}


func (s *TaskService) Get(id int) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrTaskNotFound
	}

	return *task, nil
}


func (s *TaskService) Process(id int) (Task, error) {
	input, err := s.startProcessing(id, StatusPending)
	if err != nil {
		return Task{}, err
	}

	return s.runAndComplete(id, input)
}


func (s *TaskService) changeStatus(id int, from, to Status) (*Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.Status != from {
		return nil, ErrInvalidState
	}

	task.Status = to
	return task, nil
}


func (s *TaskService) markQueued(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.changeStatus(id, StatusPending, StatusQueued)
	return err
}


func (s *TaskService) unmarkQueued(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.changeStatus(id, StatusQueued, StatusPending)
}


func (s *TaskService) startProcessing(id int, from Status) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.changeStatus(id, from, StatusProcessing)
	if err != nil {
		return "", err
	}

	return task.Input, nil
}


func (s *TaskService) runAndComplete(id int, input string) (Task, error) {
	result := s.processor.Process(input)

	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.changeStatus(id, StatusProcessing, StatusCompleted)
	if err != nil {
		return Task{}, err
	}

	task.Result = result
	return *task, nil
}
