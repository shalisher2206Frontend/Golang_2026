package main

import (
	"strings"
	"unicode/utf8"
)

type Processor interface {
	Process(text string) Result
}

type TextProcessor struct{}

func (p TextProcessor) Process(text string) Result {
	return Result{
		Words: len(strings.Fields(text)),
		Runes: utf8.RuneCountInString(text),
	}
}
