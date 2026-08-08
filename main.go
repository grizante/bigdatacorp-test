// Command bigdatacorp-test turns a JSONL file of clubs into clubs.csv and
// players.csv in the working directory.
//
// Usage:
//
//	go run . <input.jsonl>
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"bigdatacorp-test/pipeline"
)

const (
	clubsPath   = "clubs.csv"
	playersPath = "players.csv"
)

// usageError marks the two failures the spec says to answer with usage text:
// a missing argument and an input file that will not open. A write failure is
// not one of them — printing usage there would blame the caller's command line
// for something else entirely.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }

func (e usageError) Unwrap() error { return e.err }

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)

		if _, ok := errors.AsType[usageError](err); ok {
			fmt.Fprintf(os.Stderr, "usage: %s <input.jsonl>\n", filepath.Base(os.Args[0]))
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return usageError{fmt.Errorf("expected exactly one argument, got %d", len(args))}
	}

	// The input is opened before anything is created: a mistyped path must not
	// truncate the results of a previous run.
	in, err := os.Open(args[0])
	if err != nil {
		return usageError{fmt.Errorf("cannot open input: %w", err)}
	}
	defer in.Close()

	clubs, err := os.Create(clubsPath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", clubsPath, err)
	}
	defer clubs.Close()

	players, err := os.Create(playersPath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", playersPath, err)
	}
	defer players.Close()

	return pipeline.Run(in, clubs, players, os.Stderr)
}
