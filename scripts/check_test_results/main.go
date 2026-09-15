package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type testEvent struct {
	Action  string
	Package string
	Test    string
	Output  string
}

func checkResults(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	var failures []string
	passed := false
	for {
		var event testEvent
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return fmt.Errorf("decode go test output: %w", err)
		}
		if _, err := io.WriteString(output, event.Output); err != nil {
			return err
		}
		switch event.Action {
		case "fail", "build-fail":
			failures = append(failures, "test failure: "+event.Package+" "+event.Test)
		case "pass":
			passed = true
		case "skip":
			if event.Test != "" && !postgresSkip(event) {
				failures = append(failures, "unexpected test skip: "+event.Package+" "+event.Test)
			}
		}
	}
	if len(failures) != 0 {
		return errors.New(strings.Join(failures, "\n"))
	}
	if !passed {
		return errors.New("go test did not report any passing tests")
	}
	return nil
}

func postgresSkip(event testEvent) bool {
	if event.Package == "github.com/divkix/Alita_Robot/alita/db/migrations" {
		return os.Getenv("ALITA_TEST_MIGRATION_CHAIN") != "true" && os.Getenv("ALITA_TEST_DATABASE") != "true"
	}
	return event.Package == "github.com/divkix/Alita_Robot/alita/db/chats" && event.Test == "TestUpdateChat" &&
		os.Getenv("ALITA_TEST_DATABASE") != "true"
}

func main() {
	if err := checkResults(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
