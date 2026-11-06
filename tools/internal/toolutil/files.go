// Package toolutil provides file handling shared by repository development tools.
package toolutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Root finds the launcher repository from the current directory. This also
// works with go -C tools run and with a compiled tool invoked in the checkout.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && bytes.HasPrefix(data, []byte("module github.com/Viking602/YoloMCL\n")) {
			return dir, nil
		}
		if err == nil && bytes.HasPrefix(data, []byte("module github.com/Viking602/YoloMCL\r\n")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run this tool inside the YoloMCL checkout")
		}
		dir = parent
	}
}

// WriteFile publishes a complete file only after writing and closing succeeds.
func WriteFile(path string, data []byte) error {
	return WriteOutput(path, func(f *os.File) error { _, err := f.Write(data); return err })
}

func WriteOutput(path string, write func(*os.File) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mcl-tool-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = write(f)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(f.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func JSON(value any, pretty bool) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	if !pretty {
		return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
	}
	return out.Bytes(), nil
}

func WriteJSON(path string, value any) error {
	data, err := JSON(value, true)
	if err != nil {
		return err
	}
	return WriteFile(path, data)
}
