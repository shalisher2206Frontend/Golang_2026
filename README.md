# SIS Text Analyzer

An individual Go project for SIS. Checkpoint 2: Concurrent Processing.

## Requirements
- Go 1.27
- Standard library only

## Description

The application analyzes text using a worker pool.

Each task contains:
- Unique ID
- Original input text
- Current status
- Result (word count and rune count)

**Processing rules:**
- Empty input is rejected.
- Words are separated using `strings.Fields`.
- Runes are counted using `utf8.RuneCountInString`.
- Original text is preserved.

Example: `"Hello Go world"` → 3 words, 14 runes.

## Concurrency

The application uses a worker pool with:
- 3 workers
- Buffered channel with capacity of 5
- Goroutines for concurrent processing

**How it works:**

1. `Create(text)` validates and stores a new task.
2. `Submit(id)` adds the task ID to the queue and returns immediately.
3. Workers take tasks from the queue and process them.
4. After processing, the result is saved and the status becomes `Completed`.

Tasks may finish in a different order.

If the queue is full, `Submit` returns `ErrQueueFull`, and the task remains `Pending`.

## Task Statuses

```text
Pending → Queued → Processing → Completed
```

- `Pending` – task created.
- `Queued` – waiting for a worker.
- `Processing` – currently being processed.
- `Completed` – processing finished.

The original synchronous `Process()` method from Checkpoint 1 is also supported.

Invalid status transitions return `ErrInvalidState`.

## Synchronization

- `sync.RWMutex` protects tasks and their data.
- `sync.Mutex` protects the worker pool's stopped state.
- Workers safely process tasks concurrently.
- Locks are not held while text is being processed.
- `sync.WaitGroup` waits for all workers to finish.

## Stopping the Pool

`pool.Stop()`:
1. Stops accepting new tasks.
2. Closes the queue.
3. Allows workers to finish queued tasks.
4. Waits for all workers to exit.

After stopping, new submissions return `ErrPoolStopped`.

## Project Structure

- `main.go` – application example.
- `task.go` – task types and statuses.
- `processor.go` – text processing.
- `service.go` – task management.
- `pool.go` – worker pool and queue.
- `service_test.go` – service tests.
- `pool_test.go` – concurrency tests.

## Run

```bash
go run .
```

## Tests and Checks

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

## Tests

Tests cover:
- Task creation and validation.
- Correct word and rune counts.
- Worker pool startup and shutdown.
- Concurrent task processing.
- Queue capacity and full queue handling.
- Invalid and duplicate submissions.
- Thread-safe task creation and reading.
- Race conditions between `Submit` and `Stop`.
- Correct task statuses and results.

All tests use controlled channels to check concurrency without relying on `time.Sleep`.
