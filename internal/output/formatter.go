package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"
)

// FormatOutput writes data to w in the specified format.
// Supported formats: json (default), table, csv, pretty.
func FormatOutput(w io.Writer, data interface{}, format string) error {
	switch strings.ToLower(format) {
	case "json", "":
		return formatJSON(w, data)
	case "table":
		return formatTable(w, data)
	case "csv":
		return formatCSV(w, data)
	case "pretty":
		return formatPretty(w, data)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// formatJSON writes data as indented JSON.
func formatJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(data)
}

// formatTable writes data as a human-readable table.
// For maps/objects: key-value pairs. For slices of maps: tabular with headers.
func formatTable(w io.Writer, data interface{}) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	// Normalize data to a generic form via JSON round-trip.
	normalized, err := normalize(data)
	if err != nil {
		return err
	}

	switch v := normalized.(type) {
	case map[string]interface{}:
		writeKeyValueTable(tw, v)
	case []interface{}:
		if len(v) == 0 {
			fmt.Fprintln(tw, "(empty)")
		} else if firstMap, ok := v[0].(map[string]interface{}); ok {
			writeArrayTable(tw, v, keysFromMap(firstMap))
		} else {
			// Simple array: one item per line.
			for _, item := range v {
				fmt.Fprintf(tw, "%v\n", item)
			}
		}
	default:
		fmt.Fprintf(tw, "%v\n", v)
	}

	return tw.Flush()
}

// formatCSV writes data as CSV. Works best with arrays of objects.
func formatCSV(w io.Writer, data interface{}) error {
	normalized, err := normalize(data)
	if err != nil {
		return err
	}

	cw := csv.NewWriter(w)

	switch v := normalized.(type) {
	case []interface{}:
		if len(v) == 0 {
			cw.Flush()
			return cw.Error()
		}
		if firstMap, ok := v[0].(map[string]interface{}); ok {
			keys := keysFromMap(firstMap)
			// Write header row.
			if err := cw.Write(keys); err != nil {
				return err
			}
			// Write data rows.
			for _, item := range v {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				row := make([]string, len(keys))
				for i, k := range keys {
					row[i] = fmt.Sprintf("%v", m[k])
				}
				if err := cw.Write(row); err != nil {
					return err
				}
			}
		} else {
			for _, item := range v {
				if err := cw.Write([]string{fmt.Sprintf("%v", item)}); err != nil {
					return err
				}
			}
		}
	case map[string]interface{}:
		keys := keysFromMap(v)
		if err := cw.Write([]string{"key", "value"}); err != nil {
			return err
		}
		for _, k := range keys {
			if err := cw.Write([]string{k, fmt.Sprintf("%v", v[k])}); err != nil {
				return err
			}
		}
	default:
		if err := cw.Write([]string{fmt.Sprintf("%v", v)}); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

// ANSI color codes for pretty JSON output.
const (
	ansiGreen = "\033[32m"
	ansiCyan  = "\033[36m"
	ansiReset = "\033[0m"
)

// formatPretty writes data as indented JSON with ANSI colors.
// Keys are green, string values are cyan.
func formatPretty(w io.Writer, data interface{}) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	colored := colorizeJSON(string(raw))
	_, err = fmt.Fprintln(w, colored)
	return err
}

// colorizeJSON applies ANSI colors to a JSON string.
// Keys (quoted strings before ':') are green, string values are cyan.
func colorizeJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)

	i := 0
	for i < len(s) {
		if s[i] == '"' {
			// Find the closing quote, handling escapes.
			end := findClosingQuote(s, i)
			quoted := s[i : end+1]

			// Look ahead past the closing quote for ':' to determine if this is a key.
			isKey := false
			for j := end + 1; j < len(s); j++ {
				if s[j] == ' ' || s[j] == '\t' {
					continue
				}
				if s[j] == ':' {
					isKey = true
				}
				break
			}

			if isKey {
				b.WriteString(ansiGreen)
				b.WriteString(quoted)
				b.WriteString(ansiReset)
			} else {
				b.WriteString(ansiCyan)
				b.WriteString(quoted)
				b.WriteString(ansiReset)
			}
			i = end + 1
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// findClosingQuote returns the index of the closing quote for a JSON string
// starting at position start (which should be the opening quote).
func findClosingQuote(s string, start int) int {
	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // skip escaped character
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return len(s) - 1
}

// normalize converts arbitrary data to map[string]interface{} or []interface{}
// via JSON round-trip, so we can handle structs uniformly.
func normalize(data interface{}) (interface{}, error) {
	if data == nil {
		return nil, nil
	}
	// If it's already a basic type, return as-is.
	rv := reflect.ValueOf(data)
	switch rv.Kind() {
	case reflect.Map, reflect.Slice:
		// Already a map or slice — JSON round-trip to get generic types.
	case reflect.Ptr:
		if rv.IsNil() {
			return nil, nil
		}
	default:
		// Primitive types — just return.
		return data, nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// keysFromMap returns sorted keys from a map.
func keysFromMap(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// writeKeyValueTable writes a map as key-value rows.
func writeKeyValueTable(tw *tabwriter.Writer, m map[string]interface{}) {
	keys := keysFromMap(m)
	fmt.Fprintf(tw, "KEY\tVALUE\n")
	for _, k := range keys {
		fmt.Fprintf(tw, "%s\t%v\n", k, m[k])
	}
}

// writeArrayTable writes an array of maps as a table with headers.
func writeArrayTable(tw *tabwriter.Writer, items []interface{}, keys []string) {
	// Header row.
	fmt.Fprintln(tw, strings.Join(keys, "\t"))
	// Data rows.
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vals := make([]string, len(keys))
		for i, k := range keys {
			vals[i] = fmt.Sprintf("%v", m[k])
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
}
