package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	Pending    = "Pending"
	Processing = "Processing"
	Completed  = "Completed"
)

type Result struct {
	Words      int
	Characters int
}

type Task struct {
	ID     int
	Text   string
	Status string
	Result Result
}

// Analyzer describes the text processing that a store needs.
type Analyzer interface {
	Analyze(text string) Result
}

type TextAnalyzer struct{}

func (a TextAnalyzer) Analyze(text string) Result {
	return Result{
		Words:      len(strings.Fields(text)),
		Characters: utf8.RuneCountInString(text),
	}
}

type Store struct {
	tasks  map[int]Task
	nextID int
}

func NewStore() *Store {
	return &Store{
		tasks:  make(map[int]Task),
		nextID: 1,
	}
}

func (s *Store) Create(text string) (Task, error) {
	if strings.TrimSpace(text) == "" {
		return Task{}, errors.New("text cannot be empty or contain only whitespace")
	}

	task := Task{
		ID:     s.nextID,
		Text:   text,
		Status: Pending,
	}
	s.tasks[task.ID] = task
	s.nextID++
	return task, nil
}

func (s *Store) Get(id int) (Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("task %d not found", id)
	}
	return task, nil
}

func (s *Store) Process(id int, analyzer Analyzer) error {
	task, err := s.Get(id)
	if err != nil {
		return err
	}
	if task.Status != Pending {
		return fmt.Errorf("task %d must be Pending to process", id)
	}

	task.Status = Processing
	s.tasks[id] = task

	task.Result = analyzer.Analyze(task.Text)
	task.Status = Completed
	s.tasks[id] = task
	return nil
}
