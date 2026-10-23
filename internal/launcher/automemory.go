package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AutoMemoryInput is what the automatic heap size depends on.
type AutoMemoryInput struct {
	TotalMB, AvailableMB int    // physical RAM (0 when unknown)
	Minecraft            string // e.g. "1.20.1"
	Loader               string // vanilla / fabric / forge / neoforge / quilt
	Mods                 int    // .jar files in the mods folder
	Java32               bool   // 32-bit JVM
}

const (
	autoMemMin    = 1024
	autoMemMax    = 16384
	autoMem32Bit  = 1024 // 32-bit JVMs cannot reserve much more contiguous heap
	autoMemStepMB = 256
)

// AutoMemoryMB computes -Xmx:
//
//	want    = vanilla baseline, or 4 / 6 / 8 / 10 / 12 / 14 / 16 GiB
//	          for <=30 / 80 / 150 / 250 / 400 / 600 / more enabled mods
//	ceiling = min(total − max(2.5 GiB, 25 % of total), available − 1 GiB, if known)
//	result  = clamp(min(want, ceiling), 1 GiB, 16 GiB), ≤ 1 GiB for 32-bit Java,
//	          rounded down to 256 MiB.
//
// base: < 1.13 → 2 GiB, < 1.18 → 2.5 GiB, newer → 3 GiB.
func AutoMemoryMB(in AutoMemoryInput) int {
	want := 3072
	if mcBefore(in.Minecraft, 13) {
		want = 2048
	} else if mcBefore(in.Minecraft, 18) {
		want = 2560
	}
	switch strings.ToLower(in.Loader) {
	case "forge", "neoforge", "fabric", "quilt":
		want = 4096
		for _, threshold := range []int{30, 80, 150, 250, 400, 600} {
			if in.Mods > threshold {
				want += 2048
			}
		}
	}
	ceiling := autoMemMax
	if in.TotalMB > 0 {
		ceiling = min(ceiling, in.TotalMB-max(2560, in.TotalMB/4))
		if in.AvailableMB > 0 {
			// do not plan far beyond what is free now (allow reclaiming cache)
			ceiling = min(ceiling, max(in.AvailableMB-1024, min(4096, in.TotalMB/4)))
		}
	}
	v := max(autoMemMin, min(want, ceiling, autoMemMax))
	if in.Java32 {
		v = min(v, autoMem32Bit)
	}
	return v / autoMemStepMB * autoMemStepMB
}

// mcBefore reports whether a 1.x version is older than 1.minor (snapshots
// and unknown versions count as new).
func mcBefore(mc string, minor int) bool {
	parts := strings.Split(strings.TrimSpace(mc), ".")
	if len(parts) < 2 || parts[0] != "1" {
		return false
	}
	n, e := strconv.Atoi(strings.TrimFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' }))
	return e == nil && n < minor
}

// MemoryPlan reports whether versionID uses automatic memory, and the
// manual value otherwise. Chain: version → directory (with
// UseRootSettings) → launcher settings; auto is the default.
func (r Root) MemoryPlan(versionID string, settings Settings) (bool, int) {
	vc := r.VersionConfigs[versionID]
	if vc.MemoryMB > 0 {
		return false, vc.MemoryMB
	}
	if vc.MemoryAuto {
		return true, 0
	}
	if settings.UseRootSettings {
		if r.MemoryMB > 0 {
			return false, r.MemoryMB
		}
		if r.MemoryAuto {
			return true, 0
		}
	}
	if settings.MemoryMode == "manual" && settings.MemoryMB > 0 {
		return false, settings.MemoryMB
	}
	return true, 0
}

func countJars(dir string) int {
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 0
	}
	entries, _ := os.ReadDir(dir)
	n := 0
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
			n++
		}
	}
	return n
}

// AutoMemoryFor evaluates AutoMemoryMB for a version of a directory.
func (s *Service) AutoMemoryFor(r Root, versionID, javaPath string) int {
	in := AutoMemoryInput{}
	if info, e := SystemMemory(); e == nil {
		in.TotalMB, in.AvailableMB = info.TotalMB, info.AvailableMB
	}
	for _, v := range scanVersions(r.Path) {
		if v.ID == versionID {
			in.Minecraft, in.Loader = v.Minecraft, v.Loader
		}
	}
	rr := r
	rr.SelectedVersion = versionID
	in.Mods = countJars(filepath.Join(rr.VersionDir(), "mods"))
	if javaPath != "" {
		if j, e := inspectJava(context.Background(), javaPath); e == nil {
			in.Java32 = isJava32(j.Architecture)
		}
	}
	return AutoMemoryMB(in)
}

func isJava32(arch string) bool {
	a := strings.ToLower(arch)
	if a == "" || strings.Contains(a, "64") {
		return false
	}
	return strings.Contains(a, "86") || a == "arm" || strings.Contains(a, "i386")
}
