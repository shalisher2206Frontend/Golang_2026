package main

import (
	"errors"
	"sync"
)

var (
	ErrQueueFull     = errors.New("task queue is full")
	ErrPoolStopped   = errors.New("worker pool is stopped")
	ErrInvalidConfig = errors.New("workers and queue capacity must be positive")
)


type WorkerPool struct {
	service *TaskService
	queue   chan int 
	workers int

	mu      sync.Mutex
	stopped bool

	wg sync.WaitGroup 
}

func NewWorkerPool(service *TaskService, workers, queueCapacity int) (*WorkerPool, error) {
	if workers <= 0 || queueCapacity <= 0 {
		return nil, ErrInvalidConfig
	}

	p := &WorkerPool{
		service: service,
		queue:   make(chan int, queueCapacity),
		workers: workers,
	}

	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}

	return p, nil
}


func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for id := range p.queue {
		input, err := p.service.startProcessing(id, StatusQueued)
		if err != nil {
			continue
		}

		_, _ = p.service.runAndComplete(id, input)
	}
}


func (p *WorkerPool) Submit(id int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return ErrPoolStopped
	}

	if err := p.service.markQueued(id); err != nil {
		return err
	}

	select {
	case p.queue <- id:
		return nil
	default:
		p.service.unmarkQueued(id) // Queued -> Pending
		return ErrQueueFull
	}
}


func (p *WorkerPool) Stop() error {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return ErrPoolStopped
	}
	p.stopped = true
	close(p.queue)
	p.mu.Unlock()

	p.wg.Wait()
	return nil
}


func (p *WorkerPool) Workers() int {
	return p.workers
}


func (p *WorkerPool) QueueCapacity() int {
	return cap(p.queue)
}
