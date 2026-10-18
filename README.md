# Text Analyzer — SIS Checkpoint 1

A small Go 1.27 application that counts words and characters in text.
It processes one task at a time and stores tasks and results in memory.
It uses only Go's standard library.

## Files

- `go.mod`: the module name and required Go version.
- `main.go`: reads your text from the terminal, processes it, and prints the results.
- `task.go`: task data, text analysis, validation, storage, and lifecycle.
- `task_test.go`: automated tests using Go's `testing` package.
- `README.md`: project explanation and commands.

## Counting rules

- Words are separated by whitespace, using `strings.Fields`.
- Punctuation stays attached to a word: `Hello, Go!` has two words.
- Characters are counted as Unicode runes using `utf8.RuneCountInString`,
  so non-English letters are not counted as multiple bytes.
- Spaces, tabs, newlines, and punctuation count as characters.
- Some combined symbols or emoji contain more than one rune.
- Empty or whitespace-only input is rejected with an error.

For example, `Hello Go world` has 3 words and 14 characters.

## Task lifecycle

`Pending -> Processing -> Completed`

1. `Create` validates the text, gives the task a unique integer ID, and saves it as `Pending`.
2. `Process` accepts only a `Pending` task and stores its `Processing` status before analysis starts.
3. After analysis, `Process` saves the result and changes the status to `Completed`.

A completed task cannot be processed again. A task that is already processing
cannot be started again. Missing task IDs return errors. There is no public
operation for setting an arbitrary status. Processing is synchronous, so a normal
caller sees `Completed` when `Process` returns successfully.

Each task has its own ID, text, status, and result. A map stores tasks by ID.
`Get` returns a copy, so changing the returned task does not change the stored task.
All data is lost when the program stops.

## Go concepts used

- Structs: `Task`, `Result`, `Store`, and `TextAnalyzer`.
- Methods: `Create`, `Get`, `Process`, and `Analyze`.
- Pointer receivers: store methods use `*Store`; creation advances the store's ID counter.
- Value receiver: `TextAnalyzer.Analyze` does not change the analyzer.
- Interface: `Analyzer` describes the analysis method used by `Store.Process`;
  `TextAnalyzer` implements it.
- Map: `map[int]Task` holds independent tasks.
- Loop: `main` keeps reading text until you enter `/exit`.
- Errors: invalid input, missing IDs, and repeated processing return errors.
- Tests: table-driven tests check invalid input and expected analysis results.

Create stores with `NewStore()` and pass `TextAnalyzer{}` to `Process`.

## Run

Open a terminal in this folder, then run:

```sh
go run .
```

Type your own text and press Enter. Each line creates a separate task.
The program prints its ID, word count, character count, and final status.
Blank input shows an error and lets you try again. Type `/exit` to stop.
Use a terminal for interactive input and run the entire project with `go run .`.

Example:

```text
Text Analyzer - type /exit to stop.
Enter text: Hello Go world
Created task 1: Pending
Text: "Hello Go world"
Words: 3
Characters: 14
Status: Completed

Enter text: /exit
```

## Check before submission

```sh
gofmt -l .
go vet ./...
go test ./...
go build ./...
```

`gofmt -l .` should print nothing. The other commands should succeed.
Tests cover creation, invalid input, retrieval, missing IDs, processing, exact
results, final status, repeated processing, unique IDs, and independent tasks.

Commit these files, including `go.mod`, to your submission repository.
No database, external library, HTTP server, or concurrency is included.
