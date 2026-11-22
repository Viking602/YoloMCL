package launcher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func sha1Hex(b []byte) string {
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}

func TestInstallsAndAddonMutationsAllowedWhileGameRunning(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "running-pack")

	// Set up an installed Fabric 1.21.4 version in the root and mark it running.
	writeTestFile(t, filepath.Join(r.Path, "versions", "1.21.4", "1.21.4.json"), jsonBytes(t, map[string]any{
		"id":        "1.21.4",
		"type":      "release",
		"mainClass": "net.minecraft.client.main.Main",
	}))
	writeTestFile(t, filepath.Join(r.Path, "versions", "fabric-loader-0.16.0-1.21.4", "fabric-loader-0.16.0-1.21.4.json"), jsonBytes(t, map[string]any{
		"id":           "fabric-loader-0.16.0-1.21.4",
		"inheritsFrom": "1.21.4",
		"mainClass":    "net.fabricmc.loader.impl.launch.knot.KnotClient",
	}))
	if err := s.Select(r.ID, "", "fabric-loader-0.16.0-1.21.4"); err != nil {
		t.Fatal(err)
	}
	r, err := s.getRoot(r.ID)
	if err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	s.running[runKey(r.ID, r.SelectedVersion)] = &runningGame{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, runKey(r.ID, r.SelectedVersion))
		s.mu.Unlock()
	}()

	packBytes := zipBytes(t, map[string][]byte{"pack.mcmeta": []byte(`{"pack":{"description":"Faithful 64x"}}`)})
	shaderBytes := zipBytes(t, map[string][]byte{"shaders/gbuffers_basic.fsh": []byte("void main(){}")})
	modBytes := zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"demo","name":"Demo","version":"1.0.0"}`)})
	vanillaMeta := jsonBytes(t, map[string]any{
		"id":        "1.21.5",
		"type":      "release",
		"mainClass": "net.minecraft.client.main.Main",
		"downloads": map[string]any{
			"client": map[string]any{
				"url":  "https://piston-data.mojang.com/v1/objects/client.jar",
				"sha1": sha1Hex([]byte("client-jar")),
				"size": len("client-jar"),
			},
		},
	})
	manifestBytes := jsonBytes(t, map[string]any{
		"latest": map[string]any{"release": "1.21.5"},
		"versions": []map[string]any{{
			"id":   "1.21.5",
			"type": "release",
			"url":  "https://piston-meta.mojang.com/v1/packages/1.21.5.json",
			"sha1": sha1Hex(vanillaMeta),
		}},
	})

	fakeHTTP(t, s, map[string][]byte{
		"https://api.modrinth.com/v2/version/rp-ver": jsonBytes(t, map[string]any{
			"id":             "rp-ver",
			"project_id":     "rp-proj",
			"name":           "Faithful 64x",
			"version_number": "1.0",
			"game_versions":  []string{"1.21.4"},
			"files": []map[string]any{{
				"url":      "https://cdn.modrinth.com/data/rp-proj/versions/rp-ver/Faithful64x.zip",
				"filename": "Faithful64x.zip",
				"primary":  true,
				"size":     len(packBytes),
				"hashes":   map[string]string{"sha1": sha1Hex(packBytes)},
			}},
		}),
		"https://cdn.modrinth.com/data/rp-proj/versions/rp-ver/Faithful64x.zip": packBytes,
		"https://api.modrinth.com/v2/version/sh-ver": jsonBytes(t, map[string]any{
			"id":             "sh-ver",
			"project_id":     "sh-proj",
			"name":           "Glow Shader",
			"version_number": "1.0",
			"game_versions":  []string{"1.21.4"},
			"files": []map[string]any{{
				"url":      "https://cdn.modrinth.com/data/sh-proj/versions/sh-ver/GlowShader.zip",
				"filename": "GlowShader.zip",
				"primary":  true,
				"size":     len(shaderBytes),
				"hashes":   map[string]string{"sha1": sha1Hex(shaderBytes)},
			}},
		}),
		"https://cdn.modrinth.com/data/sh-proj/versions/sh-ver/GlowShader.zip": shaderBytes,
		"https://api.modrinth.com/v2/version/mod-ver": jsonBytes(t, map[string]any{
			"id":             "mod-ver",
			"project_id":     "mod-proj",
			"name":           "Demo Mod",
			"version_number": "1.0.0",
			"game_versions":  []string{"1.21.4"},
			"loaders":        []string{"fabric"},
			"files": []map[string]any{{
				"url":      "https://cdn.modrinth.com/data/mod-proj/versions/mod-ver/demo-1.0.0.jar",
				"filename": "demo-1.0.0.jar",
				"primary":  true,
				"size":     len(modBytes),
				"hashes":   map[string]string{"sha1": sha1Hex(modBytes)},
			}},
		}),
		"https://cdn.modrinth.com/data/mod-proj/versions/mod-ver/demo-1.0.0.jar": modBytes,
		"https://api.modrinth.com/v2/version_files":                              []byte(`{}`),
		"https://api.modrinth.com/v2/version_files/update":                       []byte(`{}`),
		"https://piston-meta.mojang.com/mc/game/version_manifest_v2.json":        manifestBytes,
		"https://piston-meta.mojang.com/v1/packages/1.21.5.json":                 vanillaMeta,
		"https://piston-data.mojang.com/v1/objects/client.jar":                   []byte("client-jar"),
	})

	// 1. Remote resource pack & shader pack installs succeed while running.
	rpJob, err := s.InstallAssetPack(r.ID, "resourcepack", "modrinth", "rp-proj", "rp-ver")
	if err != nil {
		t.Fatalf("InstallAssetPack(resourcepack) while running: %v", err)
	}
	if j := waitJob(t, s, rpJob); j.State != "completed" {
		t.Fatalf("resource pack job state=%s msg=%q", j.State, j.Message)
	}
	shJob, err := s.InstallAssetPack(r.ID, "shader", "modrinth", "sh-proj", "sh-ver")
	if err != nil {
		t.Fatalf("InstallAssetPack(shader) while running: %v", err)
	}
	if j := waitJob(t, s, shJob); j.State != "completed" {
		t.Fatalf("shader pack job state=%s msg=%q", j.State, j.Message)
	}

	// 2. Local resource pack import, toggle, remove-undo, and restore succeed while running.
	localPack := filepath.Join(t.TempDir(), "LocalPack.zip")
	writeTestFile(t, localPack, packBytes)
	if err := s.AddLocalAssetPacks(r.ID, "resourcepack", []string{localPack}); err != nil {
		t.Fatalf("AddLocalAssetPacks while running: %v", err)
	}
	if err := s.ToggleAssetPack(r.ID, "resourcepack", "LocalPack.zip"); err != nil {
		t.Fatalf("ToggleAssetPack while running: %v", err)
	}
	receipt, err := s.RemoveAssetPackUndo(r.ID, "", "resourcepack", "LocalPack.zip.disabled")
	if err != nil {
		t.Fatalf("RemoveAssetPackUndo while running: %v", err)
	}
	if err := s.RestoreMod(receipt); err != nil {
		t.Fatalf("RestoreMod(assetpack) while running: %v", err)
	}

	// 3. Remote mod install (InstallMod & InstallPlanned), local mod import, toggle, remove, and CheckMods succeed while running.
	modJob, err := s.InstallMod(r.ID, "modrinth", "mod-proj", "mod-ver", "")
	if err != nil {
		t.Fatalf("InstallMod while running: %v", err)
	}
	if j := waitJob(t, s, modJob); j.State != "completed" {
		t.Fatalf("InstallMod job state=%s msg=%q", j.State, j.Message)
	}
	if err := s.ToggleMod(r.ID, "demo-1.0.0.jar"); err != nil {
		t.Fatalf("ToggleMod while running: %v", err)
	}
	planJob, err := s.InstallPlanned(r.ID, nil, []string{"demo-1.0.0.jar.disabled"}, nil)
	if err != nil {
		t.Fatalf("InstallPlanned while running: %v", err)
	}
	if j := waitJob(t, s, planJob); j.State != "completed" {
		t.Fatalf("InstallPlanned job state=%s msg=%q", j.State, j.Message)
	}
	localMod := filepath.Join(t.TempDir(), "local-extra.jar")
	writeTestFile(t, localMod, modBytes)
	if err := s.AddLocalMods(r.ID, []string{localMod}); err != nil {
		t.Fatalf("AddLocalMods while running: %v", err)
	}
	if err := s.RemoveMod(r.ID, "local-extra.jar"); err != nil {
		t.Fatalf("RemoveMod while running: %v", err)
	}
	chkJob, err := s.CheckMods(r.ID)
	if err != nil {
		t.Fatalf("CheckMods while running: %v", err)
	}
	if j := waitJob(t, s, chkJob); j.State != "completed" {
		t.Fatalf("CheckMods job state=%s msg=%q", j.State, j.Message)
	}

	// 4. Installing another game version into the same root succeeds while running.
	gameJob, err := s.InstallGame(r.ID, "", "", "1.21.5", "vanilla", "")
	if err != nil {
		t.Fatalf("InstallGame while running: %v", err)
	}
	if j := waitJob(t, s, gameJob); j.State != "completed" {
		t.Fatalf("InstallGame job state=%s msg=%q", j.State, j.Message)
	}
	if _, err := os.Stat(filepath.Join(r.Path, "versions", "1.21.5", "1.21.5.jar")); err != nil {
		t.Fatalf("installed game jar missing: %v", err)
	}

	// 5. Strict operations still refuse while a game in the root is running.
	if err := s.RemoveRoot(r.ID); err == nil {
		t.Fatal("RemoveRoot succeeded while game was running")
	}
	if _, err := s.RemoveRootUndo(r.ID); err == nil {
		t.Fatal("RemoveRootUndo succeeded while game was running")
	}
	if _, err := s.ExportPack(ExportPackRequest{
		RootID:      r.ID,
		Version:     "fabric-loader-0.16.0-1.21.4",
		Format:      "modrinth",
		Name:        "Pack",
		PackVersion: "1.0.0",
		Path:        filepath.Join(t.TempDir(), "out.mrpack"),
	}); err == nil {
		t.Fatal("ExportPack succeeded while game was running")
	}
}

func TestRetryAndRootLockSemanticsWhileGameRunning(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "retry-running")

	// Start a strict job before a game runs and have it fail, then mark the game running:
	// retrying a strict job must still refuse while a game runs.
	strictID, err := s.startJob("strict task", r.ID, func(context.Context, progress) error {
		return errors.New("boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, s, strictID); j.State != "failed" {
		t.Fatalf("strict job state=%s", j.State)
	}

	s.mu.Lock()
	s.running[runKey(r.ID, "v1")] = &runningGame{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, runKey(r.ID, "v1"))
		s.mu.Unlock()
	}()

	if err := s.RetryJob(strictID); err == nil {
		t.Fatal("RetryJob on strict job succeeded while game was running")
	}

	// An allowRunning job can fail and be retried while the game is running.
	var attempts atomic.Int32
	allowID, err := s.startJobAllowRunning("install task", r.ID, func(context.Context, progress) error {
		if attempts.Add(1) == 1 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("startJobAllowRunning while running: %v", err)
	}
	if j := waitJob(t, s, allowID); j.State != "failed" {
		t.Fatalf("first attempt state=%s", j.State)
	}
	if err := s.RetryJob(allowID); err != nil {
		t.Fatalf("RetryJob on allowRunning job while running: %v", err)
	}
	if j := waitJob(t, s, allowID); j.State != "completed" {
		t.Fatalf("retried allowRunning job state=%s msg=%q", j.State, j.Message)
	}

	// Concurrent root locks still serialize allowRunning jobs and mutations.
	s.mu.Lock()
	s.rootLocks[r.ID] = true
	s.mu.Unlock()
	_, jobErr := s.startJobAllowRunning("concurrent install", r.ID, func(context.Context, progress) error { return nil })
	mutErr := s.mutateRootAllowRunning(r.ID, func(Root) error { return nil })
	s.mu.Lock()
	delete(s.rootLocks, r.ID)
	s.mu.Unlock()
	if jobErr == nil {
		t.Fatal("startJobAllowRunning bypassed active root lock")
	}
	if mutErr == nil {
		t.Fatal("mutateRootAllowRunning bypassed active root lock")
	}
	var loc interface{ LocalizedMessage(bool) string }
	if !errors.As(jobErr, &loc) || loc.LocalizedMessage(true) == "" {
		t.Fatalf("expected bilingual error for active root lock, got %T: %v", jobErr, jobErr)
	}
}
