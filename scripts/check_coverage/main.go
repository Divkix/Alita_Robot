// Package main enforces per-package and global coverage floors on a merged Go
// coverage profile.
//
// CI concatenates one profile per test shard, so the same file:position block
// can appear many times. Statement counts belong to the block, not the profile:
// they are kept once and execution counts are summed, which matches a single
// unsplit run.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	modulePrefix = "github.com/divkix/Alita_Robot/"
	globalFloor  = 78.0
)

var floors = []floor{
	{"alita/db", 70.10},
	{"alita/db/cache", 93.43},
	{"alita/enforce", 98.33},
	{"alita/i18n", 91.84},
	{"alita/settings", 94.92},
	{"alita/utils/chat_status", 90.29},
	{"alita/utils/helpers", 97.25},
	{"alita/utils/shutdown", 87.18},
}

type floor struct {
	pkg  string
	want float64
}

type block struct {
	numStmt int64
	count   int64
}

type stats struct {
	covered int64
	total   int64
}

func (s *stats) add(numStmt, count int64) {
	s.total += numStmt
	if count > 0 {
		s.covered += numStmt
	}
}

func (s *stats) percent() float64 {
	if s.total == 0 {
		return 0
	}
	return 100 * float64(s.covered) / float64(s.total)
}

type row struct {
	pkg      string
	coverage string
	floor    float64
	result   string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check_coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "coverage.out", "path to a Go coverage profile")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	content, err := os.ReadFile(*profile)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err := check(bytes.NewReader(content), stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func check(input io.Reader, output io.Writer) error {
	blocks, err := parseProfile(input)
	if err != nil {
		return err
	}
	pkgs := aggregate(blocks)

	rows := make([]row, 0, len(floors)+1)
	var failures []string
	for _, f := range floors {
		s, ok := pkgs[f.pkg]
		if !ok {
			rows = append(rows, row{f.pkg, "n/a", f.want, "missing"})
			failures = append(failures, f.pkg+": no coverage data in profile")
			continue
		}
		got := roundPercent(s.percent())
		result := "pass"
		if got < f.want {
			result = "fail"
			failures = append(failures, fmt.Sprintf("%s: %.2f%% < %.2f%%", f.pkg, got, f.want))
		}
		rows = append(rows, row{f.pkg, fmt.Sprintf("%.2f%%", got), f.want, result})
	}

	var total stats
	for _, s := range pkgs {
		total.covered += s.covered
		total.total += s.total
	}
	got := roundPercent(total.percent())
	result := "pass"
	if got < globalFloor {
		result = "fail"
		failures = append(failures, fmt.Sprintf("total: %.2f%% < %.2f%%", got, globalFloor))
	}
	rows = append(rows, row{"total", fmt.Sprintf("%.2f%%", got), globalFloor, result})

	_, _ = fmt.Fprintln(output, "| Package | Coverage | Floor | Result |")
	_, _ = fmt.Fprintln(output, "| --- | --- | --- | --- |")
	for _, r := range rows {
		_, _ = fmt.Fprintf(output, "| %s | %s | %.2f%% | %s |\n", r.pkg, r.coverage, r.floor, r.result)
	}

	if len(failures) != 0 {
		return errors.New("coverage below floor:\n  " + strings.Join(failures, "\n  "))
	}
	return nil
}

// parseProfile returns blocks keyed by file:start.end position. Duplicate keys
// keep the first statement count and sum execution counts; a differing
// statement count means the profile is corrupt, not just merged.
func parseProfile(input io.Reader) (map[string]*block, error) {
	blocks := make(map[string]*block)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "mode:") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: want 3 fields, got %q", line, text)
		}
		if colon := strings.LastIndexByte(fields[0], ':'); colon <= 0 || colon == len(fields[0])-1 {
			return nil, fmt.Errorf("line %d: malformed block %q", line, fields[0])
		}
		numStmt, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse statement count: %w", line, err)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse execution count: %w", line, err)
		}
		if existing, ok := blocks[fields[0]]; ok {
			if existing.numStmt != numStmt {
				return nil, fmt.Errorf("%s: inconsistent numStmt %d and %d", fields[0], existing.numStmt, numStmt)
			}
			existing.count += count
			continue
		}
		blocks[fields[0]] = &block{numStmt: numStmt, count: count}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return blocks, nil
}

// aggregate groups blocks by package directory, with the module prefix removed.
func aggregate(blocks map[string]*block) map[string]*stats {
	pkgs := make(map[string]*stats)
	for position, b := range blocks {
		file := position[:strings.LastIndexByte(position, ':')]
		pkg := strings.TrimPrefix(path.Dir(file), modulePrefix)
		s := pkgs[pkg]
		if s == nil {
			s = &stats{}
			pkgs[pkg] = s
		}
		s.add(b.numStmt, b.count)
	}
	return pkgs
}

func roundPercent(percent float64) float64 {
	return math.Round(percent*100) / 100
}
