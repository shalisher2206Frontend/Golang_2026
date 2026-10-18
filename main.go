package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	store := NewStore()
	analyzer := TextAnalyzer{}
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Text Analyzer - type /exit to stop.")

	for {
		fmt.Print("Enter text: ")
		if !scanner.Scan() {
			break
		}
		text := scanner.Text()
		if text == "/exit" {
			break
		}

		task, err := store.Create(text)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}
		fmt.Printf("Created task %d: %s\n", task.ID, task.Status)

		err = store.Process(task.ID, analyzer)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		task, err = store.Get(task.ID)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Printf("Text: %q\n", task.Text)
		fmt.Println("Words:", task.Result.Words)
		fmt.Println("Characters:", task.Result.Characters)
		fmt.Println("Status:", task.Status)
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading input:", err)
	}
}
