package main

import (
	"errors"
	"testing"
)

func TestCreateAndGet(t *testing.T) {
	service := NewTaskService(TextProcessor{})

	task, err := service.Create("Hello Go")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if task.ID <= 0 {
		t.Errorf("expected positive ID, got %d", task.ID)
	}

	if task.Input != "Hello Go" {
		t.Errorf("unexpected input: %q", task.Input)
	}

	if task.Status != StatusPending {
		t.Errorf("expected Pending, got %s", task.Status)
	}

	if task.Result != (Result{}) {
		t.Errorf("expected empty result, got %+v", task.Result)
	}

	stored, err := service.Get(task.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if stored != task {
		t.Errorf("stored task differs: got %+v, want %+v", stored, task)
	}

	// Changing the returned copy must not change the stored task.
	stored.Input = "changed"
	stored.Status = StatusCompleted

	again, err := service.Get(task.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if again != task {
		t.Errorf("stored task was changed through its returned copy")
	}
}

func TestCreateInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "spaces", input: "   "},
		{name: "tabs and newlines", input: "\t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTaskService(TextProcessor{})

			_, err := service.Create(tt.input)
			if !errors.Is(err, ErrEmptyInput) {
				t.Fatalf("expected ErrEmptyInput, got %v", err)
			}

			if len(service.tasks) != 0 {
				t.Errorf("invalid task was stored")
			}
		})
	}
}

func TestMissingTask(t *testing.T) {
	service := NewTaskService(TextProcessor{})

	_, err := service.Get(999)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Get: expected ErrTaskNotFound, got %v", err)
	}

	_, err = service.Process(999)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Process: expected ErrTaskNotFound, got %v", err)
	}
}

func TestProcessingResults(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Result
	}{
		{
			name:  "English",
			input: "Hello Go world",
			want:  Result{Words: 3, Runes: 14},
		},
		{
			name:  "Unicode",
			input: "Привет мир",
			want:  Result{Words: 2, Runes: 10},
		},
		{
			name:  "different whitespace",
			input: "  Go\tis\nfun  ",
			want:  Result{Words: 3, Runes: 13},
		},
		{
			name:  "punctuation",
			input: "Hello, Go!",
			want:  Result{Words: 2, Runes: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTaskService(TextProcessor{})

			task, err := service.Create(tt.input)
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}

			processed, err := service.Process(task.ID)
			if err != nil {
				t.Fatalf("Process returned error: %v", err)
			}

			if processed.Result != tt.want {
				t.Errorf("got %+v, want %+v", processed.Result, tt.want)
			}

			if processed.Status != StatusCompleted {
				t.Errorf("expected Completed, got %s", processed.Status)
			}

			stored, err := service.Get(task.ID)
			if err != nil {
				t.Fatalf("Get returned error: %v", err)
			}

			if stored != processed {
				t.Errorf("processing result was not saved correctly")
			}
		})
	}
}

func TestRejectRepeatedProcessing(t *testing.T) {
	service := NewTaskService(TextProcessor{})

	task, err := service.Create("Hello Go")
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Process(task.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Process(task.ID)
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}

	stored, err := service.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored != first {
		t.Errorf("rejected operation changed the task")
	}
}

func TestMultipleIndependentTasks(t *testing.T) {
	service := NewTaskService(TextProcessor{})

	first, err := service.Create("Go")
	if err != nil {
		t.Fatal(err)
	}

	second, err := service.Create("Hello world")
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == second.ID {
		t.Fatal("task IDs must be unique")
	}

	_, err = service.Process(first.ID)
	if err != nil {
		t.Fatal(err)
	}

	waiting, err := service.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}

	if waiting != second {
		t.Error("processing the first task changed the second task")
	}

	_, err = service.Process(second.ID)
	if err != nil {
		t.Fatal(err)
	}

	savedFirst, err := service.Get(first.ID)
	if err != nil {
		t.Fatal(err)
	}

	savedSecond, err := service.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}

	if savedFirst.Input != "Go" ||
		savedFirst.Result != (Result{Words: 1, Runes: 2}) ||
		savedFirst.Status != StatusCompleted {
		t.Errorf("incorrect first task: %+v", savedFirst)
	}

	if savedSecond.Input != "Hello world" ||
		savedSecond.Result != (Result{Words: 2, Runes: 11}) ||
		savedSecond.Status != StatusCompleted {
		t.Errorf("incorrect second task: %+v", savedSecond)
	}
}

// This test processor lets us inspect the task during processing.
type observingProcessor struct {
	observe func()
}

func (p observingProcessor) Process(text string) Result {
	p.observe()
	return Result{Words: 1, Runes: 2}
}

func TestProcessingState(t *testing.T) {
	called := false

	processor := observingProcessor{}
	service := NewTaskService(processor)

	task, err := service.Create("Go")
	if err != nil {
		t.Fatal(err)
	}

	processor.observe = func() {
		called = true

		current, err := service.Get(task.ID)
		if err != nil {
			t.Fatal(err)
		}

		if current.Status != StatusProcessing {
			t.Errorf("expected Processing, got %s", current.Status)
		}

		_, err = service.Process(task.ID)
		if !errors.Is(err, ErrInvalidState) {
			t.Errorf("expected rejection during processing, got %v", err)
		}
	}

	service.processor = processor

	completed, err := service.Process(task.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("processor was not called")
	}

	if completed.Status != StatusCompleted {
		t.Errorf("expected Completed, got %s", completed.Status)
	}
}
