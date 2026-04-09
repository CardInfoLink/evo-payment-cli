package output

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// --- Unit Tests ---

func TestFormatOutputJSON(t *testing.T) {
	data := map[string]interface{}{"name": "test", "value": 42}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "json"); err != nil {
		t.Fatalf("FormatOutput json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["name"] != "test" {
		t.Errorf("name = %v, want test", parsed["name"])
	}
}

func TestFormatOutputJSONIndented(t *testing.T) {
	data := map[string]string{"key": "val"}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "  ") {
		t.Error("expected indented JSON output")
	}
}

func TestFormatOutputTable_Map(t *testing.T) {
	data := map[string]interface{}{"name": "alice", "age": 30}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "table"); err != nil {
		t.Fatalf("FormatOutput table: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "name") || !strings.Contains(out, "age") {
		t.Errorf("table output missing keys: %s", out)
	}
}

func TestFormatOutputTable_Array(t *testing.T) {
	data := []map[string]interface{}{
		{"id": 1, "name": "alice"},
		{"id": 2, "name": "bob"},
	}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "table"); err != nil {
		t.Fatalf("FormatOutput table: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "id") || !strings.Contains(out, "name") {
		t.Errorf("table output missing headers: %s", out)
	}
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Errorf("table output missing data: %s", out)
	}
}

func TestFormatOutputCSV_Array(t *testing.T) {
	data := []map[string]interface{}{
		{"id": 1, "name": "alice"},
		{"id": 2, "name": "bob"},
	}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "csv"); err != nil {
		t.Fatalf("FormatOutput csv: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, ",") {
		t.Errorf("csv output missing commas: %s", out)
	}
	if !strings.Contains(out, "id") || !strings.Contains(out, "name") {
		t.Errorf("csv output missing headers: %s", out)
	}
}

func TestFormatOutputCSV_Map(t *testing.T) {
	data := map[string]interface{}{"key1": "val1", "key2": "val2"}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "csv"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, ",") {
		t.Errorf("csv output missing commas: %s", out)
	}
}

func TestFormatOutputPretty(t *testing.T) {
	data := map[string]interface{}{"name": "test", "value": 42}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, "pretty"); err != nil {
		t.Fatalf("FormatOutput pretty: %v", err)
	}
	out := buf.String()
	// Should contain ANSI color codes.
	if !strings.Contains(out, "\033[") {
		t.Error("pretty output missing ANSI color codes")
	}
	// Stripping ANSI codes should yield valid JSON.
	stripped := stripANSI(out)
	stripped = strings.TrimSpace(stripped)
	if !json.Valid([]byte(stripped)) {
		t.Errorf("pretty output (stripped) is not valid JSON: %s", stripped)
	}
}

func TestFormatOutputUnsupported(t *testing.T) {
	var buf bytes.Buffer
	err := FormatOutput(&buf, "data", "xml")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestFormatOutputEmptyFormat(t *testing.T) {
	data := map[string]string{"k": "v"}
	var buf bytes.Buffer
	if err := FormatOutput(&buf, data, ""); err != nil {
		t.Fatal(err)
	}
	// Empty format defaults to JSON.
	if !json.Valid(buf.Bytes()) {
		t.Error("empty format should default to JSON")
	}
}

func TestFormatOutputTable_EmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatOutput(&buf, []interface{}{}, "table"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(empty)") {
		t.Error("expected (empty) for empty array table")
	}
}

func TestFormatOutputCSV_EmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatOutput(&buf, []interface{}{}, "csv"); err != nil {
		t.Fatal(err)
	}
	// Should produce empty or minimal output without error.
}

func TestColorizeJSON(t *testing.T) {
	input := `{"key": "value"}`
	colored := colorizeJSON(input)
	if !strings.Contains(colored, ansiGreen) {
		t.Error("expected green color for keys")
	}
	if !strings.Contains(colored, ansiCyan) {
		t.Error("expected cyan color for string values")
	}
}

// --- Property 26: 输出格式切换 ---
// Feature: evo-payment-cli, Property 26: 输出格式切换
// For random data and each format (json/table/csv/pretty), FormatOutput must produce valid output.
// - json: must be valid JSON
// - table: must contain key names
// - csv: must contain comma-separated values
// - pretty: must be valid JSON (ignoring ANSI codes)
// **Validates: Requirement 7.6**

func TestProperty26_OutputFormatSwitching(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 100,
		Rand:     rand.New(rand.NewSource(42)),
	}

	// Generate random test data: map with 1-5 string key-value pairs.
	type testInput struct {
		Keys   [5]string
		Values [5]string
		Count  uint8
	}

	err := quick.Check(func(input testInput) bool {
		count := int(input.Count%5) + 1
		data := make(map[string]interface{})
		for i := 0; i < count; i++ {
			key := sanitizeKey(input.Keys[i])
			if key == "" {
				key = "k"
			}
			data[key] = input.Values[i]
		}

		formats := []string{"json", "table", "csv", "pretty"}
		for _, format := range formats {
			var buf bytes.Buffer
			if err := FormatOutput(&buf, data, format); err != nil {
				t.Logf("FormatOutput(%q) error: %v", format, err)
				return false
			}
			out := buf.String()
			if out == "" {
				t.Logf("FormatOutput(%q) produced empty output", format)
				return false
			}

			switch format {
			case "json":
				if !json.Valid([]byte(out)) {
					t.Logf("json format produced invalid JSON: %s", out)
					return false
				}
			case "table":
				// Table output must contain at least one key name.
				found := false
				for k := range data {
					if strings.Contains(out, k) {
						found = true
						break
					}
				}
				if !found {
					t.Logf("table format missing key names: %s", out)
					return false
				}
			case "csv":
				// CSV output must contain commas.
				if !strings.Contains(out, ",") {
					t.Logf("csv format missing commas: %s", out)
					return false
				}
			case "pretty":
				// Strip ANSI codes and verify valid JSON.
				stripped := strings.TrimSpace(stripANSI(out))
				if !json.Valid([]byte(stripped)) {
					t.Logf("pretty format (stripped) is not valid JSON: %s", stripped)
					return false
				}
			}
		}
		return true
	}, cfg)

	if err != nil {
		t.Errorf("Property 26 failed: %v", err)
	}
}

// TestProperty26_ArrayFormatSwitching tests format switching with array data.
func TestProperty26_ArrayFormatSwitching(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 100,
		Rand:     rand.New(rand.NewSource(99)),
	}

	type arrayInput struct {
		Keys   [3]string
		Values [3][3]string
		Rows   uint8
	}

	err := quick.Check(func(input arrayInput) bool {
		rows := int(input.Rows%3) + 1
		// Use fixed alphanumeric keys to ensure they survive sanitization
		// and remain distinct.
		keys := []string{"colA", "colB", "colC"}

		data := make([]map[string]interface{}, rows)
		for r := 0; r < rows; r++ {
			row := make(map[string]interface{})
			for c := 0; c < 3; c++ {
				row[keys[c]] = input.Values[r][c]
			}
			data[r] = row
		}

		formats := []string{"json", "table", "csv", "pretty"}
		for _, format := range formats {
			var buf bytes.Buffer
			if err := FormatOutput(&buf, data, format); err != nil {
				return false
			}
			out := buf.String()
			if out == "" {
				return false
			}

			switch format {
			case "json":
				if !json.Valid([]byte(out)) {
					return false
				}
			case "table":
				found := false
				for _, k := range keys {
					if strings.Contains(out, k) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			case "csv":
				if !strings.Contains(out, ",") {
					return false
				}
			case "pretty":
				stripped := strings.TrimSpace(stripANSI(out))
				if !json.Valid([]byte(stripped)) {
					return false
				}
			}
		}
		return true
	}, cfg)

	if err != nil {
		t.Errorf("Property 26 (array) failed: %v", err)
	}
}

// --- Helpers ---

// ansiRegex matches ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// sanitizeKey ensures a key is a valid non-empty string suitable for map keys.
// Removes control characters and limits length.
func sanitizeKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}
