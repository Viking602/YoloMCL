// Command cubiomes-benchmark verifies and measures Go against pinned C cubiomes.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/tools/internal/toolutil"
)

const revision = "e61f90580cbdd883214a8054670dacae655e59c0"
const benchmarkFilter = "^BenchmarkReference$|^BenchmarkReferenceNew$|^BenchmarkEndMoving$|^BenchmarkBuiltinDocuments$|^BenchmarkCompileSeed$"

var sources = []string{"generator.c", "biomenoise.c", "noise.c", "layers.c", "biomes.c"}
var referenceFiles = append(append([]string{}, sources...), "generator.h", "biomenoise.h", "noise.h", "layers.h", "biomes.h", "rng.h", "LICENSE", "tables/btree18.h", "tables/btree192.h", "tables/btree19.h", "tables/btree20.h", "tables/btree21wd.h")
var compilerFlags = []string{"-std=c11", "-O3", "-fwrapv", "-ffp-contract=off"}

type options struct {
	cc, reference, output, baseline string
	fetch, verifyOnly               bool
	count                           int
}
type job struct {
	name    string
	command []string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cubiomes-benchmark:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var opt options
	f := flag.NewFlagSet("cubiomes-benchmark", flag.ContinueOnError)
	f.StringVar(&opt.cc, "cc", "gcc", "GCC/Clang executable or absolute path")
	f.StringVar(&opt.reference, "reference-dir", "", "pinned C sources (default: repository .cache/seedmap-reference)")
	f.StringVar(&opt.output, "output-dir", "", "results (default: repository .cache/cubiomes-comparison)")
	f.StringVar(&opt.baseline, "baseline", "", "optional pre-optimization Go test executable")
	f.BoolVar(&opt.fetch, "fetch", false, "fetch the pinned C sources into the reference directory")
	f.BoolVar(&opt.verifyOnly, "verify-only", false, "verify C and Go hashes without timing")
	f.IntVar(&opt.count, "count", 5, "Go repetitions (C always performs five)")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 || opt.count < 1 {
		return fmt.Errorf("unexpected arguments or nonpositive --count; use --help")
	}
	root, err := toolutil.Root()
	if err != nil {
		return err
	}
	if opt.reference == "" {
		opt.reference = filepath.Join(root, ".cache", "seedmap-reference")
	}
	if opt.output == "" {
		opt.output = filepath.Join(root, ".cache", "cubiomes-comparison")
	}
	opt.reference, err = filepath.Abs(opt.reference)
	if err != nil {
		return err
	}
	opt.output, err = filepath.Abs(opt.output)
	if err != nil {
		return err
	}
	if opt.baseline != "" {
		opt.baseline, err = filepath.Abs(opt.baseline)
		if err != nil {
			return err
		}
	}
	cc, err := exec.LookPath(opt.cc)
	if err != nil {
		return fmt.Errorf("compiler: %w", err)
	}
	opt.cc, err = filepath.Abs(cc)
	if err != nil {
		return err
	}
	if opt.fetch {
		fmt.Println("Fetching pinned cubiomes", revision)
		if err := fetchReference(&http.Client{Timeout: 30 * time.Second}, "https://raw.githubusercontent.com/Cubitect/cubiomes/"+revision+"/", opt.reference); err != nil {
			return err
		}
	}
	for _, name := range referenceFiles {
		info, err := os.Stat(filepath.Join(opt.reference, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("missing reference %s; use --fetch with a dedicated --reference-dir", name)
		}
	}
	if err := os.MkdirAll(opt.output, 0755); err != nil {
		return err
	}
	exe := filepath.Join(opt.output, "cubiomes-benchmark")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	compile := append([]string{opt.cc}, compilerFlags...)
	compile = append(compile, "-I", opt.reference, filepath.Join(root, "cubiomes", "testdata", "benchmark.c"))
	for _, name := range sources {
		compile = append(compile, filepath.Join(opt.reference, name))
	}
	compile = append(compile, "-lm", "-o", exe)
	if err := command(root, os.Stdout, compile...); err != nil {
		return err
	}
	actual, err := capture(root, exe, "--hashes")
	if err != nil {
		return err
	}
	expected, err := os.ReadFile(filepath.Join(root, "cubiomes", "testdata", "benchmark-hashes.json"))
	if err != nil {
		return err
	}
	if err := verifyHashes(actual, expected); err != nil {
		return err
	}
	if err := command(root, os.Stdout, "go", "test", "./cubiomes", "-run=^TestReferenceBenchmarkHashes$", "-count=1"); err != nil {
		return err
	}
	if opt.verifyOnly {
		fmt.Println("C and Go agree on all benchmark hashes.")
		return nil
	}
	compiler, err := capture(root, opt.cc, "--version")
	if err != nil {
		return err
	}
	compilerLine, _, _ := strings.Cut(string(compiler), "\n")
	goVersion, err := capture(root, "go", "version")
	if err != nil {
		return err
	}
	processor := os.Getenv("PROCESSOR_IDENTIFIER")
	if processor == "" {
		processor = runtime.GOARCH
	}
	meta := map[string]any{
		"date_utc": time.Now().UTC().Format(time.RFC3339Nano), "platform": runtime.GOOS + "-" + runtime.GOARCH, "processor": processor,
		"c_revision": revision, "c_flags": compilerFlags, "c_compiler": strings.TrimSpace(compilerLine), "go_version": strings.TrimSpace(string(goVersion)),
		"go_count": opt.count, "go_benchtime": "500ms", "seed": 42,
		"range_blocks": map[string]int{"x": 8192, "y": 64, "z": -8192, "width": 128, "height": 128, "step": 4},
	}
	if opt.baseline != "" {
		data, err := os.ReadFile(opt.baseline)
		if err != nil {
			return err
		}
		meta["baseline_sha256"] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	if err := toolutil.WriteJSON(filepath.Join(opt.output, "metadata.json"), meta); err != nil {
		return err
	}
	// Timing jobs deliberately run serially so they do not compete for CPU.
	for _, job := range jobs(exe, opt.baseline, opt.count) {
		fmt.Println("Measuring", job.name)
		if err := toolutil.WriteOutput(filepath.Join(opt.output, job.name), func(f *os.File) error { return command(root, f, job.command...) }); err != nil {
			return err
		}
	}
	fmt.Println("Results:", opt.output)
	return nil
}

func fetchReference(client *http.Client, baseURL, dir string) error {
	files := map[string][]byte{}
	for _, name := range referenceFiles {
		response, err := client.Get(baseURL + name)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, (16<<20)+1))
		closeErr := response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("fetch %s: HTTP %s", name, response.Status)
		}
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) > 16<<20 {
			return fmt.Errorf("reference file %s exceeds 16 MiB", name)
		}
		files[name] = data
	}
	for _, name := range referenceFiles {
		if err := toolutil.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), files[name]); err != nil {
			return err
		}
	}
	return toolutil.WriteFile(filepath.Join(dir, "reference-revision.txt"), []byte(revision+"\n"))
}

func verifyHashes(actual, expected []byte) error {
	var a, b map[string]string
	if err := json.Unmarshal(bytes.TrimPrefix(actual, []byte{0xef, 0xbb, 0xbf}), &a); err != nil {
		return fmt.Errorf("reference C hashes: %w", err)
	}
	if err := json.Unmarshal(bytes.TrimPrefix(expected, []byte{0xef, 0xbb, 0xbf}), &b); err != nil {
		return fmt.Errorf("reference hashes: %w", err)
	}
	if len(b) == 0 || !reflect.DeepEqual(a, b) {
		return fmt.Errorf("reference C output differs from checked-in hashes; check source revision and compiler flags")
	}
	return nil
}

func command(root string, stdout io.Writer, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(args[0]), err)
	}
	return nil
}

func capture(root string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	err := command(root, &out, args...)
	return out.Bytes(), err
}

func jobs(exe, baseline string, count int) []job {
	bench := func(pkg, filter, duration string) []string {
		return []string{"go", "test", pkg, "-run=^$", "-bench=" + filter, "-benchmem", "-benchtime=" + duration, "-count=" + strconv.Itoa(count)}
	}
	result := []job{
		{"c.csv", []string{exe}}, {"c-init.csv", []string{exe, "--init"}},
		{"go.txt", bench("./cubiomes", benchmarkFilter, "500ms")},
		{"mcl.txt", bench("./internal/launcher", "^BenchmarkLegacyTile$|^BenchmarkMultiNoiseTile$", "500ms")},
		{"latest.txt", bench("./cubiomes", "^BenchmarkLatest$", "500ms")},
		{"viewport.txt", bench("./internal/launcher", "^BenchmarkSeedMapViewportFirstPaint$", "1s")},
	}
	if baseline != "" {
		result = append([]job{{"before.txt", []string{baseline, "-test.run=^$", "-test.bench=" + benchmarkFilter, "-test.benchmem", "-test.benchtime=500ms", "-test.count=" + strconv.Itoa(count)}}}, result...)
	}
	return result
}
