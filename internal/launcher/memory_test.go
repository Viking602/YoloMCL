package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSystemMemoryAndSettingsLimits(t *testing.T) {
	info, e := SystemMemory()
	if e != nil {
		if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
			t.Skip(e)
		}
		t.Fatal(e)
	}
	if info.TotalMB < 512 || info.AvailableMB < 0 || info.AvailableMB > info.TotalMB || info.MaxMB > info.TotalMB || info.MaxMB > 131072 || info.RecommendedMB < 512 || info.RecommendedMB > info.MaxMB {
		t.Fatalf("invalid physical memory measurement: %+v", info)
	}
	s := testService(t)
	settings := Settings{MemoryMB: min(4096, info.MaxMB), UseRootSettings: true, ShowSnapshots: true}
	if e = s.SaveSettings(settings); e != nil {
		t.Fatal(e)
	}
	restored, e := New(s.dir)
	if e != nil || restored.settings() != settings {
		t.Fatalf("settings did not persist: %v", e)
	}
	if e = s.SaveSettings(Settings{MemoryMB: info.MaxMB + 512}); e == nil {
		t.Fatal("memory exceeding the physical-memory limit was accepted")
	}
	if s.settings() != settings {
		t.Fatal("invalid memory changed the saved settings")
	}
}

func TestExistingRootSettingsMigration(t *testing.T) {
	dir := t.TempDir()
	old := map[string]any{"schema": 1, "settings": map[string]any{"memoryMB": 2048}, "roots": []Root{{ID: "old", JavaPath: "legacy-java", MemoryMB: 1024}}}
	writeTestFile(t, filepath.Join(dir, "config.json"), jsonBytes(t, old))
	s, e := New(dir)
	if e != nil || !s.settings().UseRootSettings {
		t.Fatalf("existing directory overrides were disabled: %v", e)
	}
	old["settings"] = map[string]any{"memoryMB": 2048, "useRootSettings": false}
	b, _ := json.Marshal(old)
	writeTestFile(t, filepath.Join(dir, "config.json"), b)
	s, e = New(dir)
	if e != nil || s.settings().UseRootSettings {
		t.Fatalf("explicit global settings preference was ignored: %v", e)
	}
}

func TestPerVersionRuntimePrecedenceAndJVMArgs(t *testing.T) {
	s := testService(t)
	game := filepath.Join(t.TempDir(), "mcroot")
	v1Dir := filepath.Join(game, "versions", "1.21.1-Fabric")
	v2Dir := filepath.Join(game, "versions", "1.12.2-Forge")
	writeTestFile(t, filepath.Join(v1Dir, "1.21.1-Fabric.json"), []byte(`{"id":"1.21.1-Fabric","inheritsFrom":"1.21.1","mainClass":"net.fabricmc.loader.impl.launch.knot.KnotClient","javaVersion":{"majorVersion":21}}`))
	writeTestFile(t, filepath.Join(v2Dir, "custom-meta.json"), []byte(`{"id":"1.12.2-forge","inheritsFrom":"1.12.2","mainClass":"net.minecraft.launchwrapper.Launch","javaVersion":{"majorVersion":8}}`))

	r, err := s.AddRoot(game, "MultiVersionRoot")
	if err != nil {
		t.Fatal(err)
	}
	wantInfo, wantErr := os.Stat(game)
	gotInfo, gotErr := os.Stat(r.Path)
	if wantErr != nil || gotErr != nil || !os.SameFile(wantInfo, gotInfo) {
		t.Fatalf("expected direct versions root %q, got %q", game, r.Path)
	}
	if err = s.UpdateVersionConfig(r.ID, "1.21.1-Fabric", "C:/jdk21/bin/javaw.exe", 4096, `-XX:+UseG1GC -Dcustom.name="My World"`); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateVersionConfig(r.ID, "1.12.2-Forge", "C:/jdk8/bin/javaw.exe", 2048, `-XX:+UseConcMarkSweepGC`); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if len(snap.Instances) != 1 || len(snap.Instances[0].Versions) != 2 {
		t.Fatalf("expected 2 versions (including custom-named json), got: %+v", snap.Instances)
	}
	inst := snap.Instances[0]
	java1, mem1, jvm1, err := inst.Root.ResolveRuntime("1.21.1-Fabric", snap.Settings)
	if err != nil || java1 != "C:/jdk21/bin/javaw.exe" || mem1 != 4096 || len(jvm1) != 2 || jvm1[1] != "-Dcustom.name=My World" {
		t.Fatalf("unexpected 1.21.1 runtime resolution: java=%q mem=%d jvm=%v err=%v", java1, mem1, jvm1, err)
	}
	java2, mem2, jvm2, err := inst.Root.ResolveRuntime("1.12.2-Forge", snap.Settings)
	if err != nil || java2 != "C:/jdk8/bin/javaw.exe" || mem2 != 2048 || len(jvm2) != 1 || jvm2[0] != "-XX:+UseConcMarkSweepGC" {
		t.Fatalf("unexpected 1.12.2 runtime resolution: java=%q mem=%d jvm=%v err=%v", java2, mem2, jvm2, err)
	}
	if _, err := ParseJVMArgs("-cp evil.jar"); err == nil {
		t.Fatal("expected -cp override in custom JVM args to be rejected")
	}
}
