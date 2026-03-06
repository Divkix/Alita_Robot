package debug_bot

import (
	"strings"
	"testing"
)

func TestPrettyPrintStructSimple(t *testing.T) {
	t.Parallel()

	type Simple struct {
		Name string
		Age  int
	}

	result := PrettyPrintStruct(Simple{Name: "Alice", Age: 30})

	if !strings.Contains(result, `"Name"`) {
		t.Errorf("expected result to contain %q, got: %s", "Name", result)
	}
	if !strings.Contains(result, `"Alice"`) {
		t.Errorf("expected result to contain %q, got: %s", "Alice", result)
	}
	if !strings.Contains(result, `"Age"`) {
		t.Errorf("expected result to contain %q, got: %s", "Age", result)
	}
	if !strings.Contains(result, "30") {
		t.Errorf("expected result to contain %q, got: %s", "30", result)
	}
}

func TestPrettyPrintStructNested(t *testing.T) {
	t.Parallel()

	type Inner struct {
		Value string
	}
	type Outer struct {
		Label string
		Inner Inner
	}

	result := PrettyPrintStruct(Outer{Label: "outer", Inner: Inner{Value: "inner"}})

	if !strings.Contains(result, `"Label"`) {
		t.Errorf("expected result to contain %q, got: %s", "Label", result)
	}
	if !strings.Contains(result, `"Inner"`) {
		t.Errorf("expected result to contain %q, got: %s", "Inner", result)
	}
	if !strings.Contains(result, `"Value"`) {
		t.Errorf("expected result to contain %q, got: %s", "Value", result)
	}
	// Indented JSON should contain at least one leading space for nested fields
	if !strings.Contains(result, "  ") {
		t.Errorf("expected indented JSON with spaces, got: %s", result)
	}
}

func TestPrettyPrintStructEmpty(t *testing.T) {
	t.Parallel()

	type Empty struct{}

	result := PrettyPrintStruct(Empty{})

	if result != "{}" {
		t.Errorf("expected %q, got: %q", "{}", result)
	}
}

func TestPrettyPrintStructNil(t *testing.T) {
	t.Parallel()

	result := PrettyPrintStruct(nil)

	if result != "null" {
		t.Errorf("expected %q, got: %q", "null", result)
	}
}

func TestPrettyPrintStructMap(t *testing.T) {
	t.Parallel()

	m := map[string]int{
		"a": 1,
		"b": 2,
	}

	result := PrettyPrintStruct(m)

	if !strings.Contains(result, `"a"`) {
		t.Errorf("expected result to contain key %q, got: %s", "a", result)
	}
	if !strings.Contains(result, `"b"`) {
		t.Errorf("expected result to contain key %q, got: %s", "b", result)
	}
	if !strings.Contains(result, "1") {
		t.Errorf("expected result to contain value %q, got: %s", "1", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("expected result to contain value %q, got: %s", "2", result)
	}
}

func TestPrettyPrintStructUnmarshalable(t *testing.T) {
	t.Parallel()

	// Channels cannot be marshaled to JSON, triggering the error branch.
	ch := make(chan int)

	result := PrettyPrintStruct(ch)

	if !strings.Contains(result, "[DebugBot]") {
		t.Errorf("expected error message to contain %q, got: %q", "[DebugBot]", result)
	}
}
