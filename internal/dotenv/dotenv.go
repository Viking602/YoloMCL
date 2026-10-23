// Package dotenv reads explicitly selected configuration keys as literal text.
// It never expands variables, executes shell expressions, or discovers files.
package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Parse reads selected keys from r. Duplicate keys use the last value, and
// unrelated lines are ignored. Error messages never include configuration values.
func Parse(r io.Reader, keys ...string) (map[string]string, error) {
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	values := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
			line = strings.TrimSpace(line[len("export"):])
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !allowed[key] {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
			end := strings.IndexByte(value[1:], value[0])
			if end < 0 {
				return nil, fmt.Errorf("invalid quoted configuration at line %d", lineNumber)
			}
			end++
			tail := strings.TrimSpace(value[end+1:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				return nil, fmt.Errorf("unexpected text after configuration at line %d", lineNumber)
			}
			value = value[1:end]
		} else {
			if strings.HasPrefix(value, "#") {
				value = ""
			}
			for i := 1; i < len(value); i++ {
				if value[i] == '#' && (value[i-1] == ' ' || value[i-1] == '\t') {
					value = strings.TrimSpace(value[:i])
					break
				}
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	return values, nil
}
