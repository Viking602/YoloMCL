// Command check runs the repository's formatting, architecture and Go checks.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const modulePath = "github.com/Viking602/YoloMCL"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(1)
	}
}

func run() error {
	fix := flag.Bool("fix", false, "Apply goimports formatting before checking")
	tests := flag.Bool("tests", true, "Run development and production tests")
	cross := flag.Bool("cross", false, "Also vet and build all supported OS/architecture targets")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if _, err := os.Stat("tools/go.mod"); err != nil {
		return errors.New("run from the repository root")
	}
	files, err := sourceFiles()
	if err != nil {
		return err
	}
	mode := "-l"
	if *fix {
		mode = "-w"
		if err := normalizeImportSpacing(files); err != nil {
			return err
		}
	}
	// Batch file names to stay under Windows' command-line length limit.
	for start := 0; start < len(files); start += 40 {
		args := []string{"tool", "-modfile=tools/go.mod", "goimports", "-local", modulePath, mode}
		args = append(args, files[start:min(start+40, len(files))]...)
		if err := expectCleanOutput(args...); err != nil {
			return fmt.Errorf("formatting (apply with go run ./cmd/check -fix): %w", err)
		}
	}
	fmt.Printf("Formatting: %d Go files checked\n", len(files))
	if err := checkImportOrder(files); err != nil {
		return err
	}
	if err := checkArchitecture(files); err != nil {
		return err
	}
	for _, args := range [][]string{{"mod", "tidy", "-diff"}, {"-C", "tools", "mod", "tidy", "-diff"}} {
		if err := expectCleanOutput(args...); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"mod", "verify"},
		{"-C", "tools", "mod", "verify"},
		{"vet", "./..."},
		{"tool", "-modfile=tools/go.mod", "staticcheck", "./..."},
		{"tool", "-modfile=tools/go.mod", "staticcheck", "-tags=mygo_noinspector", "./..."},
	} {
		if err := command(nil, args...); err != nil {
			return err
		}
	}
	if *tests {
		for _, args := range [][]string{{"test", "./..."}, {"test", "-tags=mygo_noinspector", "./..."}} {
			if err := command(nil, args...); err != nil {
				return err
			}
		}
	}
	if *cross {
		for _, goos := range []string{"windows", "darwin", "linux"} {
			for _, arch := range []string{"amd64", "arm64"} {
				fmt.Printf("Cross-platform: %s/%s\n", goos, arch)
				env := []string{"GOOS=" + goos, "GOARCH=" + arch, "CGO_ENABLED=0"}
				for _, args := range [][]string{
					{"vet", "./..."},
					{"build", "./..."},
					{"build", "-tags=mygo_noinspector", "./..."},
				} {
					if err := command(env, args...); err != nil {
						return err
					}
				}
			}
		}
	}
	fmt.Println("All requested checks passed.")
	return nil
}

func sourceFiles() ([]string, error) {
	var files []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != "." && (strings.HasPrefix(entry.Name(), ".") || path == "build" || entry.Name() == "vendor" || entry.Name() == "node_modules" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// goimports preserves existing import sections. Remove empty separators first
// so -fix can also repair a local section accidentally placed before third-party
// imports; goimports then inserts the canonical section breaks again.
func normalizeImportSpacing(files []string) error {
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, data, parser.ImportsOnly)
		if err != nil {
			return err
		}
		original := append([]byte(nil), data...)
		for i := len(file.Decls) - 1; i >= 0; i-- {
			decl, ok := file.Decls[i].(*ast.GenDecl)
			if !ok || decl.Tok != token.IMPORT || !decl.Lparen.IsValid() {
				continue
			}
			start, end := positions.Position(decl.Pos()).Offset, positions.Position(decl.End()).Offset
			var lines [][]byte
			for _, line := range bytes.Split(data[start:end], []byte("\n")) {
				if len(bytes.TrimSpace(line)) > 0 {
					lines = append(lines, line)
				}
			}
			replacement := bytes.Join(lines, []byte("\n"))
			updated := append([]byte(nil), data[:start]...)
			updated = append(updated, replacement...)
			data = append(updated, data[end:]...)
		}
		if !bytes.Equal(original, data) {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkImportOrder(files []string) error {
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		previous := 0
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			group := 0
			first, _, _ := strings.Cut(name, "/")
			if strings.Contains(first, ".") {
				group = 1
			}
			if name == modulePath || strings.HasPrefix(name, modulePath+"/") {
				group = 2
			}
			if group < previous {
				return fmt.Errorf("%s: imports must be standard library, third-party, then local; run go run ./cmd/check -fix", path)
			}
			previous = group
		}
	}
	return nil
}

func expectCleanOutput(args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		_, _ = os.Stdout.Write(out)
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	if len(bytes.TrimSpace(out)) != 0 {
		_, _ = os.Stdout.Write(out)
		return errors.New("files require normalization")
	}
	return nil
}

func command(env []string, args ...string) error {
	fmt.Println("go", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func checkArchitecture(files []string) error {
	allowed := map[string][]string{
		"internal/launcher":     {"internal/distribution"},
		"internal/nativeui":     {"internal/launcher", "internal/buildinfo"},
		"internal/distribution": {"internal/dotenv"},
		"internal/diagnostics":  {"internal/launcher"},
		"internal/buildconfig":  {"internal/distribution", "internal/dotenv", "internal/buildinfo"},
		"internal/buildinfo":    {},
		"internal/release":      {"internal/buildinfo"},
		"internal/catalogrelay": {},
		"internal/dotenv":       {},
		"plugins/startup":       {},
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		pkg := filepath.ToSlash(filepath.Dir(path))
		dependencies, checked := allowed[pkg]
		if !checked {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(name, "github.com/egoist/mygo") && pkg != "internal/nativeui" && pkg != "plugins/startup" {
				return fmt.Errorf("%s: UI dependency belongs in nativeui or the startup plugin", path)
			}
			if own, ok := strings.CutPrefix(name, modulePath+"/"); ok {
				permitted := false
				for _, dependency := range dependencies {
					permitted = permitted || own == dependency
				}
				if !permitted {
					return fmt.Errorf("%s: unexpected dependency on %s; review docs/architecture.md", path, own)
				}
			}
		}
	}
	fmt.Println("Architecture: package boundaries checked")
	return nil
}
