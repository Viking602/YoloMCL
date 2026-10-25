package nativeui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type switchProcessUsage struct {
	cpu                 uint64
	cycles              uint64
	private, workingSet uint64
}

var (
	switchProcessTimes  = syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessTimes")
	switchProcessCycles = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryProcessCycleTime")
	switchProcessMemory = syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
)

func readSwitchProcessUsage(t *testing.T) switchProcessUsage {
	t.Helper()
	var created, exited, kernel, user syscall.Filetime
	if ok, _, err := switchProcessTimes.Call(^uintptr(0), uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); ok == 0 {
		t.Fatal(err)
	}
	var cycles uint64
	if ok, _, err := switchProcessCycles.Call(^uintptr(0), uintptr(unsafe.Pointer(&cycles))); ok == 0 {
		t.Fatal(err)
	}
	var counters struct {
		Size, Faults                                                                          uint32
		PeakWS, WS, PeakPaged, Paged, PeakNonpaged, Nonpaged, Pagefile, PeakPagefile, Private uintptr
	}
	counters.Size = uint32(unsafe.Sizeof(counters))
	if ok, _, err := switchProcessMemory.Call(^uintptr(0), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size)); ok == 0 {
		t.Fatal(err)
	}
	return switchProcessUsage{
		cpu:     (uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)) + (uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)),
		cycles:  cycles,
		private: uint64(counters.Private), workingSet: uint64(counters.WS),
	}
}

// Opt-in local profiling of actual packs with the real view and software
// renderer. Only the temporary launcher configuration is written; installed
// games and the user's saved selection are left alone. CPU time is measured
// by Windows, independently of the Go CPU profiler's samples of native waits.
func TestLocalPackOverviewPerformanceQA(t *testing.T) {
	game := os.Getenv("MCL_LOCAL_INSTANCE_ROOT")
	versions := strings.Split(os.Getenv("MCL_LOCAL_INSTANCE_VERSIONS"), "|")
	if game == "" || len(versions) < 2 {
		t.Skip("local pack performance QA not requested")
	}
	data := t.TempDir()
	config := map[string]any{
		"schema": 1, "activeRoot": "qa", "settings": map[string]any{"language": "zh-CN", "hardwareAcceleration": false},
		"roots": []launcher.Root{{ID: "qa", Name: "Performance QA", Path: game, SelectedVersion: versions[0]}},
	}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "config.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := launcher.New(data)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.ShowPage("home")
	tt := ui.NewTester(a.View, 1536, 1024)
	step := func() {
		if len(a.events) > 0 {
			tt.Frame()
		}
		tt.Advance(time.Second / 60)
	}
	for deadline := time.Now().Add(10 * time.Second); a.statusLoading || a.instStatus.StoragePending || a.instStatus.Dir == ""; {
		if time.Now().After(deadline) {
			t.Fatal("initial overview did not finish")
		}
		step()
		time.Sleep(time.Second / 60)
	}
	tt.Advance(time.Second)
	var rows []map[string]any
	for round := 0; round < 3; round++ {
		for _, version := range append(versions[1:], versions[0]) {
			before := readSwitchProcessUsage(t)
			var beforeHeap, afterHeap runtime.MemStats
			runtime.ReadMemStats(&beforeHeap)
			start, frames := time.Now(), a.frameSerial.Load()
			peakPrivate, peakWS := before.private, before.workingSet
			a.selectVersion("qa", version)
			tt.Frame()
			ready := time.Duration(0)
			for time.Since(start) < 850*time.Millisecond || ready == 0 {
				if time.Since(start) > 15*time.Second {
					t.Fatalf("overview %q did not finish: %s", version, a.instStatusErr)
				}
				step()
				if ready == 0 && !a.statusLoading && !a.instStatus.StoragePending && a.instStatusKey == "qa|"+version && a.instStatus.Dir != "" {
					ready = time.Since(start)
				}
				usage := readSwitchProcessUsage(t)
				peakPrivate, peakWS = max(peakPrivate, usage.private), max(peakWS, usage.workingSet)
				time.Sleep(time.Second / 60)
			}
			after := readSwitchProcessUsage(t)
			runtime.ReadMemStats(&afterHeap)
			row := map[string]any{
				"round": round, "version": version, "mods": a.instStatus.Mods,
				"ready_ms": float64(ready.Microseconds()) / 1000, "wall_ms": float64(time.Since(start).Microseconds()) / 1000,
				"cpu_ms": float64(after.cpu-before.cpu) / 1e4, "frames": a.frameSerial.Load() - frames,
				"private_peak": peakPrivate, "working_set_peak": peakWS, "private_end": after.private,
				"heap_end": afterHeap.HeapAlloc, "allocated": afterHeap.TotalAlloc - beforeHeap.TotalAlloc,
			}
			rows = append(rows, row)
			b, _ := json.Marshal(row)
			t.Log(string(b))
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
