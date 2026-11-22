# SIS Text Analyzer

An individual Go project for SIS. Checkpoint 2: concurrent processing.

## Requirements

- Go 1.27
- Standard library only

## Description

The application creates text analysis tasks, stores them in memory and
processes them with a worker pool. Each task has:
- a unique integer ID within a service instance;
- the original input text;
- a status;
- a result containing word and rune counts.

## Processing rules (from Checkpoint 1)

- Empty and whitespace-only input is rejected.
- Words are separated using `strings.Fields`; punctuation stays part of a word.
- Runes are counted using `utf8.RuneCountInString` (spaces, punctuation,
  tabs and newlines are counted too).
- The original input is preserved.

Example: `"Hello Go world"` -> Words: 3, Runes: 14.

## Concurrency architecture

```text
  Create(text) ──> TaskService (map[int]*Task, RWMutex)
                         ^              ^
  Submit(id) ──> queue: chan int        |  startProcessing / runAndComplete
                 (capacity 5)           |
                    |                   |
          +---------+---------+         |
          v         v         v         |
      Worker 1  Worker 2  Worker 3 -----+
```

1. `service.Create(text)` validates the text and stores a `Pending` task.
2. `pool.Submit(id)` moves the task to `Queued`, puts its ID into the queue
   channel and returns immediately. It does not wait for processing.
3. A fixed number of workers (`workerCount = 3` in `main.go`) read IDs from
   the queue with `for id := range queue`. A worker marks the task
   `Processing`, runs the processor, saves the result and marks it
   `Completed`. Then it takes the next task.
4. The order in which tasks finish is not guaranteed.

### Queue

- The queue is a buffered channel with a fixed capacity
  (`queueCapacity = 5` in `main.go`).
- **Full queue:** `Submit` does not block. It returns `ErrQueueFull` and the
  task goes back to `Pending`, so it can be submitted later.
- No extra goroutines are created per task; only the workers exist.

### Task lifecycle

```text
Pending ──Submit──> Queued ──worker──> Processing ──> Completed
   ^                  |
   +---queue full-----+

Pending ──Process (synchronous, Checkpoint 1)──> Processing ──> Completed
```

Allowed transitions:
- `Pending -> Queued` (Submit);
- `Queued -> Pending` (queue was full, submission rejected);
- `Queued -> Processing` (a worker took the task);
- `Pending -> Processing` (synchronous `Process`, kept from Checkpoint 1);
- `Processing -> Completed` (result saved).

Everything else returns `ErrInvalidState`: submitting a `Queued`,
`Processing` or `Completed` task, or calling `Process` on a queued task.
Rejected operations do not change the task. Retries are not supported.
The text processor cannot fail, so there is no `Failed` state.

### Synchronization

- `TaskService.mu` (`sync.RWMutex`) protects the task map, the ID counter and
  **all fields of every stored task**. Task pointers never leave the service;
  `Get` and `Create` return copies.
- `Get` uses a read lock, so many readers can work at the same time.
- Every status change is "check current status + set new status" inside one
  lock (`changeStatus`). Two goroutines can't both move the same task out of
  `Pending`/`Queued`, so a task is never processed twice.
- The lock is **not** held while the processor runs. The worker copies the
  input under the lock, processes it without the lock, and then saves the
  result under the lock. Other tasks can be created and read meanwhile.
- Workers process different task IDs, so a worker only writes the result
  into its own task.
- `WorkerPool.mu` (`sync.Mutex`) protects the `stopped` flag. `Submit` checks
  the flag and sends to the channel under this lock, and `Stop` sets the flag
  and closes the channel under the same lock. So `Submit` can never send to a
  closed channel. The send is non-blocking, so the lock is held only briefly.
- Lock order is always `WorkerPool.mu -> TaskService.mu`, never the reverse,
  so there is no deadlock.

### Stopping workers

`pool.Stop()`:
1. sets `stopped = true` - new `Submit` calls return `ErrPoolStopped`;
2. closes the queue (the pool owns the channel and is the only closer);
3. workers finish the tasks still in the queue (`range` reads remaining
   values after close) and then exit;
4. waits for all workers with `sync.WaitGroup`.

After `Stop` returns, every accepted task is `Completed`. Calling `Stop`
again returns `ErrPoolStopped` and does nothing.

## Files

- `main.go` - demonstration
- `task.go` - domain types and statuses
- `processor.go` - processing interface and text analyzer
- `service.go` - task storage and thread-safe state changes
- `pool.go` - worker pool, bounded queue, shutdown
- `service_test.go` - Checkpoint 1 tests
- `pool_test.go` - concurrency tests

## Run

```bash
go run .
```

## Check

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

## Tests

Checkpoint 1 tests (creation, validation, results, states, repeated
processing) still pass. New tests in `pool_test.go` cover:
- pool startup and invalid configuration;
- asynchronous submission (`Submit` returns while the task is processing);
- `Queued` state while all workers are busy;
- processing of many tasks with correct results;
- real overlap: 3 tasks are inside the processor at the same time, and never
  more than the number of workers;
- invalid and duplicate submissions (missing, Queued, Processing, Completed);
- full queue: `ErrQueueFull`, task stays `Pending`, can be resubmitted;
- reading tasks while others are processing;
- many goroutines creating, submitting and reading tasks at once;
- `Stop` waits for accepted tasks and workers terminate;
- `Submit` after `Stop`, and a second `Stop`;
- `Submit` racing with several `Stop` calls: no panic, no lost tasks.

Tests use a blocking test processor and channels to control timing, so they
do not depend on `time.Sleep` or on the order in which tasks finish.
