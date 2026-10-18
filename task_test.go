package main

import "testing"

func TestCreateAndGet(t *testing.T) {
	store := NewStore()
	task, err := store.Create("Hello Go world")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != 1 || task.Text != "Hello Go world" || task.Status != Pending {
		t.Fatalf("unexpected new task: %+v", task)
	}
	if task.Result != (Result{}) {
		t.Fatalf("new task should have an empty result: %+v", task.Result)
	}

	saved, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved != task {
		t.Fatalf("got %+v, want %+v", saved, task)
	}
}

func TestInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"spaces", "   "},
		{"tabs and newlines", "\t\n "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := NewStore()
			_, err := store.Create(test.text)
			if err == nil {
				t.Fatal("expected an error for blank text")
			}
			if len(store.tasks) != 0 || store.nextID != 1 {
				t.Fatal("invalid input changed the store")
			}
		})
	}
}

func TestMissingTask(t *testing.T) {
	store := NewStore()
	_, err := store.Get(99)
	if err == nil {
		t.Fatal("expected an error for a missing task")
	}
	if err := store.Process(99, TextAnalyzer{}); err == nil {
		t.Fatal("expected an error when processing a missing task")
	}
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Result
	}{
		{"simple text", "Hello Go world", Result{Words: 3, Characters: 14}},
		{"whitespace", "  Go\trocks\n", Result{Words: 2, Characters: 11}},
		{"unicode", "Привет Go", Result{Words: 2, Characters: 9}},
		{"punctuation", "Go is fun!", Result{Words: 3, Characters: 10}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := NewStore()
			task, err := store.Create(test.text)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Process(task.ID, TextAnalyzer{}); err != nil {
				t.Fatal(err)
			}
			saved, err := store.Get(task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Result != test.want {
				t.Errorf("got %+v, want %+v", saved.Result, test.want)
			}
			if saved.Status != Completed {
				t.Errorf("got status %s, want %s", saved.Status, Completed)
			}
			if saved.ID != task.ID || saved.Text != test.text {
				t.Error("processing changed the task ID or input")
			}
		})
	}
}

func TestCannotProcessTwice(t *testing.T) {
	store := NewStore()
	task, err := store.Create("Hello Go")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Process(task.ID, TextAnalyzer{}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Process(task.ID, TextAnalyzer{}); err == nil {
		t.Fatal("expected an error when processing a completed task")
	}
	after, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("rejected operation changed the task")
	}
}

func TestIndependentTasks(t *testing.T) {
	store := NewStore()
	first, err := store.Create("Hello Go")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create("World")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("tasks must have different IDs")
	}
	if err := store.Process(first.ID, TextAnalyzer{}); err != nil {
		t.Fatal(err)
	}
	savedSecond, err := store.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if savedSecond != second {
		t.Fatal("processing the first task changed the second task")
	}
	if err := store.Process(second.ID, TextAnalyzer{}); err != nil {
		t.Fatal(err)
	}
	savedFirst, err := store.Get(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	savedSecond, err = store.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if savedFirst.Result != (Result{Words: 2, Characters: 8}) || savedFirst.Status != Completed {
		t.Fatalf("unexpected first task: %+v", savedFirst)
	}
	if savedSecond.Result != (Result{Words: 1, Characters: 5}) || savedSecond.Status != Completed {
		t.Fatalf("unexpected second task: %+v", savedSecond)
	}
}
