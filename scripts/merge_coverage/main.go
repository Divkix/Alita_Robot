// Command merge_coverage combines atomic Go coverage profiles without losing uncovered blocks.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
)

var blockLine = regexp.MustCompile(`^(.+):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+) ([0-9]+) ([0-9]+)$`)

type coverageBlock struct {
	numStmt int64
	count   int64
}

func parseBlock(text string) (string, coverageBlock, error) {
	fields := blockLine.FindStringSubmatch(text)
	if fields == nil {
		return "", coverageBlock{}, errors.New("invalid coverage block")
	}
	var values [6]int64
	for i, field := range fields[2:] {
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return "", coverageBlock{}, fmt.Errorf("invalid coverage number: %w", err)
		}
		values[i] = value
	}
	if values[0] == 0 || values[1] == 0 || values[2] == 0 || values[3] == 0 ||
		values[2] < values[0] || (values[2] == values[0] && values[3] < values[1]) {
		return "", coverageBlock{}, errors.New("invalid coverage coordinates")
	}
	key := fmt.Sprintf("%s:%d.%d,%d.%d", fields[1], values[0], values[1], values[2], values[3])
	return key, coverageBlock{numStmt: values[4], count: values[5]}, nil
}

func readProfile(path string, blocks map[string]coverageBlock) error {
	profile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = profile.Close() }()

	scanner := bufio.NewScanner(profile)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return fmt.Errorf("%s: missing coverage mode", path)
	}
	if scanner.Text() != "mode: atomic" {
		return fmt.Errorf("%s: expected mode: atomic, got %q", path, scanner.Text())
	}
	line := 1
	for scanner.Scan() {
		line++
		key, block, err := parseBlock(scanner.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, line, err)
		}
		previous, exists := blocks[key]
		if exists && previous.numStmt != block.numStmt {
			return fmt.Errorf("%s:%d: statement count mismatch for %s", path, line, key)
		}
		if block.count > math.MaxInt64-previous.count {
			return fmt.Errorf("%s:%d: coverage count overflow for %s", path, line, key)
		}
		blocks[key] = coverageBlock{numStmt: block.numStmt, count: previous.count + block.count}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if line == 1 {
		return fmt.Errorf("%s: no coverage blocks", path)
	}
	return nil
}

func mergeCoverage(paths []string, output io.Writer) error {
	if len(paths) == 0 {
		return errors.New("usage: merge_coverage <coverage.out> [coverage.out ...]")
	}
	blocks := make(map[string]coverageBlock)
	for _, path := range paths {
		if err := readProfile(path, blocks); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(blocks))
	for key := range blocks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writer := bufio.NewWriter(output)
	if _, err := fmt.Fprintln(writer, "mode: atomic"); err != nil {
		return err
	}
	for _, key := range keys {
		block := blocks[key]
		if _, err := fmt.Fprintf(writer, "%s %d %d\n", key, block.numStmt, block.count); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func main() {
	if err := mergeCoverage(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
