package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingProcessor lets a test control processing:
// each call reports to started and waits until release is closed.
// It also tracks how many calls run at the same time.
type blockingProcessor struct {
	started chan string
	release chan struct{}

	active    atomic.Int32
	maxActive atomic.Int32
}

func newBlockingProcessor() *blockingProcessor {
	return &blockingProcessor{
		started: make(chan string, 100),
		release: make(chan struct{}),
	}
}

func (p *blockingProcessor) Process(text string) Result {
	now := p.active.Add(1)
	for {
		old := p.maxActive.Load()
		if now <= old || p.maxActive.CompareAndSwap(old, now) {
			break
		}
	}

	p.started <- text
	<-p.release

	p.active.Add(-1)
	return TextProcessor{}.Process(text)
}

// waitStarted waits until n processor calls have started, or fails the test.
// The timeout only protects the test from hanging forever on a bug.
func waitStarted(t *testing.T, p *blockingProcessor, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-p.started:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d tasks started processing", i, n)
		}
	}
}

func newPool(t *testing.T, service *TaskService, workers, capacity int) *WorkerPool {
	t.Helper()
	pool, err := NewWorkerPool(service, workers, capacity)
	if err != nil {
		t.Fatalf("NewWorkerPool returned error: %v", err)
	}
	return pool
}

func mustCreate(t *testing.T, service *TaskService, text string) Task {
	t.Helper()
	task, err := service.Create(text)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	return task
}

func mustGet(t *testing.T, service *TaskService, id int) Task {
	t.Helper()
	task, err := service.Get(id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	return task
}

func TestPoolStartup(t *testing.T) {
	service := NewTaskService(TextProcessor{})
	pool := newPool(t, service, 3, 5)

	if pool.Workers() != 3 {
		t.Errorf("expected 3 workers, got %d", pool.Workers())
	}
	if pool.QueueCapacity() != 5 {
		t.Errorf("expected capacity 5, got %d", pool.QueueCapacity())
	}

	if err := pool.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestPoolInvalidConfig(t *testing.T) {
	service := NewTaskService(TextProcessor{})

	tests := []struct{ workers, capacity int }{
		{0, 5}, {3, 0}, {-1, 5}, {3, -1},
	}
	for _, tt := range tests {
		_, err := NewWorkerPool(service, tt.workers, tt.capacity)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("workers=%d capacity=%d: expected ErrInvalidConfig, got %v",
				tt.workers, tt.capacity, err)
		}
	}
}

func TestSubmitIsAsynchronous(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 2, 5)

	task := mustCreate(t, service, "Hello Go world")

	// Submit must return while the processor is still blocked.
	if err := pool.Submit(task.ID); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	waitStarted(t, processor, 1)

	if got := mustGet(t, service, task.ID); got.Status != StatusProcessing {
		t.Errorf("expected Processing, got %s", got.Status)
	}

	close(processor.release)
	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	got := mustGet(t, service, task.ID)
	if got.Status != StatusCompleted {
		t.Errorf("expected Completed, got %s", got.Status)
	}
	if got.Result != (Result{Words: 3, Runes: 14}) {
		t.Errorf("wrong result: %+v", got.Result)
	}
}

func TestQueuedState(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 2, 5)

	// Two tasks occupy both workers, the third one must wait in the queue.
	a := mustCreate(t, service, "a")
	b := mustCreate(t, service, "b")
	c := mustCreate(t, service, "c")

	for _, id := range []int{a.ID, b.ID} {
		if err := pool.Submit(id); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, processor, 2)

	if err := pool.Submit(c.ID); err != nil {
		t.Fatal(err)
	}
	if got := mustGet(t, service, c.ID); got.Status != StatusQueued {
		t.Errorf("expected Queued, got %s", got.Status)
	}

	close(processor.release)
	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	for _, id := range []int{a.ID, b.ID, c.ID} {
		if got := mustGet(t, service, id); got.Status != StatusCompleted {
			t.Errorf("task %d: expected Completed, got %s", id, got.Status)
		}
	}
}

func TestProcessManyTasks(t *testing.T) {
	service := NewTaskService(TextProcessor{})
	pool := newPool(t, service, 3, 50)

	want := make(map[int]Result)
	for i := 1; i <= 30; i++ {
		// Task i contains i words: "w w w ...".
		text := strings.TrimSpace(strings.Repeat("w ", i))
		task := mustCreate(t, service, text)

		if err := pool.Submit(task.ID); err != nil {
			t.Fatalf("Submit task %d: %v", task.ID, err)
		}
		want[task.ID] = Result{Words: i, Runes: len(text)}
	}

	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	// Order of completion does not matter, only the final state and result.
	for id, result := range want {
		got := mustGet(t, service, id)
		if got.Status != StatusCompleted {
			t.Errorf("task %d: expected Completed, got %s", id, got.Status)
		}
		if got.Result != result {
			t.Errorf("task %d: got %+v, want %+v", id, got.Result, result)
		}
	}
}

func TestTasksAreProcessedConcurrently(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 3, 10)

	var ids []int
	for i := 0; i < 6; i++ {
		task := mustCreate(t, service, fmt.Sprintf("task %d", i))
		if err := pool.Submit(task.ID); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}

	// All 3 workers are inside Process at the same time, and none of them
	// can finish until release is closed. This proves real overlap.
	waitStarted(t, processor, 3)

	if active := processor.active.Load(); active != 3 {
		t.Errorf("expected 3 tasks processing at once, got %d", active)
	}

	processing := 0
	for _, id := range ids {
		if mustGet(t, service, id).Status == StatusProcessing {
			processing++
		}
	}
	if processing != 3 {
		t.Errorf("expected 3 Processing tasks, got %d", processing)
	}

	close(processor.release)
	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	// The number of workers limits concurrency.
	if peak := processor.maxActive.Load(); peak > 3 {
		t.Errorf("more tasks than workers ran at once: %d", peak)
	}

	for _, id := range ids {
		if got := mustGet(t, service, id); got.Status != StatusCompleted {
			t.Errorf("task %d: expected Completed, got %s", id, got.Status)
		}
	}
}

func TestInvalidSubmissions(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 2, 5)

	// Missing task.
	if err := pool.Submit(999); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("missing task: expected ErrTaskNotFound, got %v", err)
	}

	// Duplicate submission while Processing.
	processing := mustCreate(t, service, "processing")
	if err := pool.Submit(processing.ID); err != nil {
		t.Fatal(err)
	}
	waitStarted(t, processor, 1)
	if err := pool.Submit(processing.ID); !errors.Is(err, ErrInvalidState) {
		t.Errorf("Processing task: expected ErrInvalidState, got %v", err)
	}

	// Duplicate submission while Queued (second worker is busy too).
	busy := mustCreate(t, service, "busy")
	if err := pool.Submit(busy.ID); err != nil {
		t.Fatal(err)
	}
	waitStarted(t, processor, 1)

	queued := mustCreate(t, service, "queued")
	if err := pool.Submit(queued.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Submit(queued.ID); !errors.Is(err, ErrInvalidState) {
		t.Errorf("Queued task: expected ErrInvalidState, got %v", err)
	}

	// The synchronous Process from Checkpoint 1 must not take a queued task.
	if _, err := service.Process(queued.ID); !errors.Is(err, ErrInvalidState) {
		t.Errorf("Process of Queued task: expected ErrInvalidState, got %v", err)
	}

	close(processor.release)
	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	// Duplicate submission of a Completed task (using a new pool).
	pool2 := newPool(t, service, 2, 5)
	defer pool2.Stop()

	before := mustGet(t, service, queued.ID)
	if before.Status != StatusCompleted {
		t.Fatalf("expected Completed, got %s", before.Status)
	}
	if err := pool2.Submit(queued.ID); !errors.Is(err, ErrInvalidState) {
		t.Errorf("Completed task: expected ErrInvalidState, got %v", err)
	}
	if after := mustGet(t, service, queued.ID); after != before {
		t.Errorf("rejected submission changed the task")
	}
}

func TestQueueFull(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 2, 1)

	// Both workers are busy (submit one by one: the queue holds only 1 task)...
	for _, text := range []string{"one", "two"} {
		task := mustCreate(t, service, text)
		if err := pool.Submit(task.ID); err != nil {
			t.Fatal(err)
		}
		waitStarted(t, processor, 1)
	}

	// ...the single queue slot is taken...
	waiting := mustCreate(t, service, "waiting")
	if err := pool.Submit(waiting.ID); err != nil {
		t.Fatalf("expected free queue slot, got %v", err)
	}

	// ...so the next submission is rejected.
	rejected := mustCreate(t, service, "rejected")
	if err := pool.Submit(rejected.ID); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	// The rejected task stays Pending, not Queued or Processing.
	if got := mustGet(t, service, rejected.ID); got.Status != StatusPending {
		t.Errorf("rejected task: expected Pending, got %s", got.Status)
	}

	// Free the workers. Space appears and the same task can be submitted again.
	close(processor.release)
	resubmitted := false
	for i := 0; i < 1000 && !resubmitted; i++ {
		err := pool.Submit(rejected.ID)
		switch {
		case err == nil:
			resubmitted = true
		case errors.Is(err, ErrQueueFull):
			time.Sleep(time.Millisecond) // only to retry; result does not depend on it
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if !resubmitted {
		t.Fatal("queue never had free space")
	}

	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	for _, id := range []int{waiting.ID, rejected.ID} {
		if got := mustGet(t, service, id); got.Status != StatusCompleted {
			t.Errorf("task %d: expected Completed, got %s", id, got.Status)
		}
	}
}

func TestGetWhileProcessing(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 2, 5)

	busy := mustCreate(t, service, "busy task")
	other := mustCreate(t, service, "other task")

	if err := pool.Submit(busy.ID); err != nil {
		t.Fatal(err)
	}
	waitStarted(t, processor, 1)

	// While a worker is processing, reads and creates must not be blocked
	// (the service lock is not held during processing).
	if got := mustGet(t, service, busy.ID); got.Status != StatusProcessing {
		t.Errorf("expected Processing, got %s", got.Status)
	}
	if got := mustGet(t, service, other.ID); got != other {
		t.Errorf("other task changed: %+v", got)
	}
	mustCreate(t, service, "created during processing")

	close(processor.release)
	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	service := NewTaskService(TextProcessor{})
	pool := newPool(t, service, 4, 200)

	const goroutines = 10
	const perGoroutine = 15

	type submitted struct {
		id   int
		text string
	}

	results := make(chan submitted, goroutines*perGoroutine)
	var wg sync.WaitGroup

	// Many goroutines create, submit and read tasks at the same time.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				text := fmt.Sprintf("goroutine %d task %d", g, i)
				task, err := service.Create(text)
				if err != nil {
					t.Errorf("Create: %v", err)
					return
				}
				if err := pool.Submit(task.ID); err != nil {
					t.Errorf("Submit: %v", err)
					return
				}
				if _, err := service.Get(task.ID); err != nil {
					t.Errorf("Get: %v", err)
				}
				results <- submitted{id: task.ID, text: text}
			}
		}(g)
	}

	wg.Wait()
	close(results)

	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	seen := make(map[int]bool)
	for r := range results {
		if seen[r.id] {
			t.Errorf("duplicate task ID %d", r.id)
		}
		seen[r.id] = true

		got := mustGet(t, service, r.id)
		if got.Input != r.text {
			t.Errorf("task %d: input %q, want %q", r.id, got.Input, r.text)
		}
		if got.Status != StatusCompleted {
			t.Errorf("task %d: expected Completed, got %s", r.id, got.Status)
		}
		// Each result must belong to its own task: "goroutine G task I" = 4 words.
		if got.Result != (Result{Words: 4, Runes: len(r.text)}) {
			t.Errorf("task %d: wrong result %+v", r.id, got.Result)
		}
	}

	if len(seen) != goroutines*perGoroutine {
		t.Errorf("expected %d tasks, got %d", goroutines*perGoroutine, len(seen))
	}
}

func TestStopFinishesAcceptedTasks(t *testing.T) {
	processor := newBlockingProcessor()
	service := NewTaskService(processor)
	pool := newPool(t, service, 2, 10)

	var ids []int
	for i := 0; i < 8; i++ {
		task := mustCreate(t, service, fmt.Sprintf("task %d", i))
		if err := pool.Submit(task.ID); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}
	waitStarted(t, processor, 2)

	stopped := make(chan error)
	go func() { stopped <- pool.Stop() }()

	// Stop must wait for the workers, which are still blocked.
	select {
	case <-stopped:
		t.Fatal("Stop returned before accepted tasks were processed")
	case <-time.After(50 * time.Millisecond):
	}

	close(processor.release)

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return: workers did not terminate")
	}

	// No accepted task was lost.
	for _, id := range ids {
		if got := mustGet(t, service, id); got.Status != StatusCompleted {
			t.Errorf("task %d: expected Completed, got %s", id, got.Status)
		}
	}
}

func TestSubmitAfterStop(t *testing.T) {
	service := NewTaskService(TextProcessor{})
	pool := newPool(t, service, 2, 5)

	if err := pool.Stop(); err != nil {
		t.Fatal(err)
	}

	task := mustCreate(t, service, "too late")
	if err := pool.Submit(task.ID); !errors.Is(err, ErrPoolStopped) {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}

	if got := mustGet(t, service, task.ID); got.Status != StatusPending {
		t.Errorf("rejected task: expected Pending, got %s", got.Status)
	}

	// The second Stop is safe and reports that the pool is already stopped.
	if err := pool.Stop(); !errors.Is(err, ErrPoolStopped) {
		t.Errorf("second Stop: expected ErrPoolStopped, got %v", err)
	}
}

func TestSubmitDuringStop(t *testing.T) {
	service := NewTaskService(TextProcessor{})
	pool := newPool(t, service, 3, 100)

	const n = 100
	ids := make([]int, n)
	for i := range ids {
		ids[i] = mustCreate(t, service, fmt.Sprintf("text %d", i)).ID
	}

	errs := make([]error, n)
	var wg sync.WaitGroup

	// Submit and Stop race with each other. There must be no panic
	// (send on closed channel) and every task must end in a valid state.
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = pool.Submit(ids[i])
		}(i)
	}

	var stopWG sync.WaitGroup
	for i := 0; i < 3; i++ {
		stopWG.Add(1)
		go func() {
			defer stopWG.Done()
			_ = pool.Stop() // several concurrent Stop calls are safe too
		}()
	}

	wg.Wait()
	stopWG.Wait()

	for i, id := range ids {
		got := mustGet(t, service, id)
		switch {
		case errs[i] == nil:
			// Accepted before stop => must be processed.
			if got.Status != StatusCompleted {
				t.Errorf("accepted task %d: expected Completed, got %s", id, got.Status)
			}
		case errors.Is(errs[i], ErrPoolStopped):
			if got.Status != StatusPending {
				t.Errorf("rejected task %d: expected Pending, got %s", id, got.Status)
			}
		default:
			t.Errorf("task %d: unexpected error %v", id, errs[i])
		}
	}
}
