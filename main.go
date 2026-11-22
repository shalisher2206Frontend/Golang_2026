package main

import (
	"errors"
	"fmt"
)

const (
	workerCount   = 3
	queueCapacity = 5
)

func main() {
	service := NewTaskService(TextProcessor{})

	pool, err := NewWorkerPool(service, workerCount, queueCapacity)
	if err != nil {
		fmt.Println("Pool error:", err)
		return
	}
	fmt.Printf("Started pool: workers=%d, queue capacity=%d\n",
		pool.Workers(), pool.QueueCapacity())

	texts := []string{
		"Hello Go world",
		"Привет мир",
		"Go is fun",
		"Concurrency is not parallelism",
	}

	var ids []int
	for _, text := range texts {
		task, err := service.Create(text)
		if err != nil {
			fmt.Println("Create error:", err)
			continue
		}

		if err := pool.Submit(task.ID); err != nil {
			fmt.Printf("Submit task %d error: %v\n", task.ID, err)
			continue
		}

		ids = append(ids, task.ID)
		fmt.Printf("Submitted task %d: %q\n", task.ID, task.Input)
	}

	if _, err := service.Create("   "); err != nil {
		fmt.Println("Expected validation error:", err)
	}
	if err := pool.Submit(999); errors.Is(err, ErrTaskNotFound) {
		fmt.Println("Expected lookup error:", err)
	}

	if err := pool.Stop(); err != nil {
		fmt.Println("Stop error:", err)
	}
	fmt.Println("Pool stopped")

	for _, id := range ids {
		task, err := service.Get(id)
		if err != nil {
			fmt.Println("Get error:", err)
			continue
		}
		fmt.Printf("Task %d: status=%s, words=%d, runes=%d\n",
			task.ID, task.Status, task.Result.Words, task.Result.Runes)
	}

	if err := pool.Submit(ids[0]); err != nil {
		fmt.Println("Expected submit after stop error:", err)
	}

	late, _ := service.Create("submitted too late")
	if err := pool.Submit(late.ID); err != nil {
		fmt.Println("Expected submit after stop error:", err)
	}

	if err := pool.Stop(); err != nil {
		fmt.Println("Expected second stop error:", err)
	}
}
