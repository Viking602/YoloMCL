package nativeui

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Opt-in, windowless startup/idle measurements using real instance metadata.
// The input is a separate config containing only roots and settings. Copies of
// the inventory/Java caches go into a temporary data directory; game files and
// the running launcher's configuration are never written. Network update
// dialogs are excluded, but Java discovery, snapshots, inventory maintenance,
// artwork, animations and idle memory reclamation use the production paths.
// Temporary-file cleanup is excluded unless a read-only build overlay is used
// with MCL_IDLE_QA_MAINTENANCE=readonly-overlay: normal Maintain can delete old
// partial downloads in the real instance directories.
func TestLocalPackIdlePerformanceQA(t *testing.T) {
	input := os.Getenv("MCL_IDLE_QA_CONFIG")
	if input == "" {
		t.Skip("local idle performance QA not requested")
	}
	data := t.TempDir()
	copyFile := func(src, dst string) {
		t.Helper()
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(input, filepath.Join(data, "config.json"))
	if cache := os.Getenv("MCL_IDLE_QA_CACHE"); cache != "" {
		copyFile(filepath.Join(cache, "java-list.json"), filepath.Join(data, "cache", "java-list.json"))
		files, err := filepath.Glob(filepath.Join(cache, "instances", "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			copyFile(file, filepath.Join(data, "cache", "instances", filepath.Base(file)))
		}
	}
	var a *App
	var tt *ui.Tester
	updates := make(chan func(), 256)
	var rows []map[string]any
	for _, phase := range []struct {
		name     string
		duration time.Duration
	}{{"startup", 8 * time.Second}, {"idle", 22 * time.Second}} {
		var profile *os.File
		if dir := os.Getenv("MCL_IDLE_QA_PROFILE"); dir != "" {
			var err error
			profile, err = os.Create(filepath.Join(dir, phase.name+".cpu"))
			if err != nil {
				t.Fatal(err)
			}
			if err := pprof.StartCPUProfile(profile); err != nil {
				profile.Close()
				t.Fatal(err)
			}
		}
		before := readSwitchProcessUsage(t)
		var beforeHeap, afterHeap runtime.MemStats
		runtime.ReadMemStats(&beforeHeap)
		start := time.Now()
		frames := uint64(0)
		updateCount := 0
		firstPaint, overviewReady := time.Duration(0), time.Duration(0)
		if a == nil {
			PreloadAssets()
			s, err := launcher.New(data)
			if err != nil {
				t.Fatal(err)
			}
			a, err = New(s)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { a.cancel(); a.bgWG.Wait() }()
			a.updateUI = func(fn func()) {
				select {
				case updates <- fn:
				case <-a.ctx.Done():
				}
			}
			a.firstFrame = make(chan struct{})
			a.ShowPage("home")
			tt = ui.NewTester(a.View, 1536, 1024)
			firstPaint = time.Since(start)
			close(a.firstFrame)
			a.startBackgroundJava()
			a.bgWG.Add(2)
			go a.refreshInventories()
			go func() { defer a.bgWG.Done(); a.pollSnapshots(nil) }()
			if os.Getenv("MCL_IDLE_QA_MAINTENANCE") == "readonly-overlay" {
				a.bgWG.Add(1)
				go func() { defer a.bgWG.Done(); s.Maintain(a.ctx) }()
			}
		} else {
			frames = a.frameSerial.Load()
		}
		peakPrivate, peakWS := before.private, before.workingSet
		lastSample, lastCPU, peakCPU := start, before.cpu, float64(0)
		lastCycles, peakCycles := before.cycles, float64(0)
		lastStep := tt.Now()
		for time.Since(start) < phase.duration {
			changed := false
			for len(updates) > 0 {
				(<-updates)()
				updateCount++
				changed = true
			}
			if changed {
				tt.Frame()
			}
			now := time.Now()
			tt.Advance(now.Sub(lastStep))
			lastStep = now
			// Sampling must include the rendering work above in both CPU
			// time and elapsed wall time, including a slow first frame.
			now = time.Now()
			if phase.name == "startup" && overviewReady == 0 && !a.statusLoading && a.instStatus.Dir != "" {
				overviewReady = now.Sub(start)
			}
			if now.Sub(lastSample) >= 250*time.Millisecond {
				usage := readSwitchProcessUsage(t)
				peakPrivate, peakWS = max(peakPrivate, usage.private), max(peakWS, usage.workingSet)
				peakCPU = max(peakCPU, float64(usage.cpu-lastCPU)/1e7/now.Sub(lastSample).Seconds()*100/float64(runtime.NumCPU()))
				peakCycles = max(peakCycles, float64(usage.cycles-lastCycles)/now.Sub(lastSample).Seconds())
				lastCycles = usage.cycles
				lastSample, lastCPU = now, usage.cpu
			}
			// Once settled, do not make the measurement harness itself wake
			// the runtime at 60 Hz. Advance still services all scheduled UI
			// frames, so an accidental idle animation remains observable.
			delay := time.Second / 60
			if phase.name == "idle" {
				delay = 250 * time.Millisecond
			}
			time.Sleep(delay)
		}
		after := readSwitchProcessUsage(t)
		runtime.ReadMemStats(&afterHeap)
		row := map[string]any{
			"phase": phase.name, "wall_ms": float64(time.Since(start).Microseconds()) / 1000,
			"cpu_ms": float64(after.cpu-before.cpu) / 1e4, "peak_machine_cpu_percent": peakCPU,
			"cpu_cycles": after.cycles - before.cycles, "peak_cycles_per_second": peakCycles, "ui_updates": updateCount,
			"frames": a.frameSerial.Load() - frames, "private_peak": peakPrivate, "working_set_peak": peakWS,
			"private_end": after.private, "heap_end": afterHeap.HeapAlloc, "allocated": afterHeap.TotalAlloc - beforeHeap.TotalAlloc,
			"java_scanning": a.javaScanning, "status_loading": a.statusLoading, "mods": a.instStatus.Mods,
			"first_paint_ms": float64(firstPaint.Microseconds()) / 1000, "overview_ready_ms": float64(overviewReady.Microseconds()) / 1000,
		}
		rows = append(rows, row)
		b, _ := json.Marshal(row)
		t.Log(string(b))
		if profile != nil {
			pprof.StopCPUProfile()
			profile.Close()
		}
		if phase.name == "startup" && os.Getenv("MCL_IDLE_QA_STARTUP_ONLY") == "1" {
			break
		}
	}
	// PNG encoding is QA output, not launcher work. Keep its temporary image
	// and encoder buffers outside both the startup and idle measurements.
	if path := os.Getenv("MCL_IDLE_QA_IMAGE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if path := os.Getenv("MCL_LOCAL_PERF_REPORT"); path != "" {
		b, err := json.MarshalIndent(rows, "", "  ")
		if err == nil {
			err = os.WriteFile(path, b, 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}
