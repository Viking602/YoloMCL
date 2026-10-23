// Command build builds the launcher with publisher settings on any host.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Viking602/YoloMCL/internal/buildconfig"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run() error {
	release := flag.Bool("release", false, "Build without the UI inspector")
	output := flag.String("output", "", "Output file (relative to repository root)")
	publisher := flag.String("publisher-config", "", "Explicit publisher JSON file")
	goos := flag.String("goos", "", "Target operating system")
	goarch := flag.String("goarch", "", "Target architecture")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("run from the repository root: %w", err)
	}
	data, err := exec.Command("go", "env", "-json", "GOOS", "GOARCH", "GOFLAGS").Output()
	if err != nil {
		return fmt.Errorf("read Go target settings: %w", err)
	}
	var env map[string]string
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	if *goos == "" {
		*goos = env["GOOS"]
	}
	if *goarch == "" {
		*goarch = env["GOARCH"]
	}
	if *output == "" {
		*output = filepath.Join(".cache", "MCL-dev")
		if *goos == "windows" {
			*output += ".exe"
		}
	}
	config, err := buildconfig.Load(root, *publisher, os.Getenv)
	if err != nil {
		return err
	}
	args, err := buildconfig.Arguments(config, *goos, env["GOFLAGS"], *output, *release)
	if err != nil {
		return err
	}
	metadata, err := buildconfig.Metadata(root)
	if err != nil {
		return err
	}
	args, err = buildconfig.MetadataArguments(args, metadata, *goos, *goarch, *release && metadata.Version != "dev")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return err
	}
	fmt.Printf("Publisher configuration: Microsoft=%t, CurseForge=%t\n", config.HasMicrosoft(), config.HasCurseForge())
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+*goos, "GOARCH="+*goarch)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile launcher: %w", err)
	}
	path, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	fmt.Println("Native launcher:", path)
	return nil
}
