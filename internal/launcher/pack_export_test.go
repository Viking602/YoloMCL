package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func packExportFixture(t *testing.T, s *Service) Root {
	t.Helper()
	r := newTestRoot(t, s, "Export fixture")
	installFabricFixture(t, s, r)
	r, err := s.getRoot(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(r.Path, "versions", r.SelectedVersion)
	for name, value := range map[string]string{
		"mods/local.jar": "local mod", "mods/off.jar.disabled": "disabled mod", "config/pack.toml": "custom=true", "kubejs/server_scripts/example.js": "// script",
		"kubejs/assets/test/lang/en_us.json": "{}", "config/library/assets/file.bin": "nested asset",
		"saves/World/level.dat": "world", "options.txt": "fullscreen:false", "screenshots/private.png": "screenshot", "servers.dat": "server",
		"logs/latest.log": "log", ".mcl/mods.json": "{}", "launcher_accounts.json": "private account", "assets/test": "asset",
	} {
		writeTestFile(t, filepath.Join(base, filepath.FromSlash(name)), []byte(value))
	}
	// A second version and shared root must not leak into an isolated export.
	writeTestFile(t, filepath.Join(r.Path, "mods/other.jar"), []byte("another instance"))
	return r
}

func exportRequest(t *testing.T, r Root, format string) ExportPackRequest {
	t.Helper()
	ext := ".mrpack"
	if format == "curseforge" {
		ext = ".zip"
	}
	return ExportPackRequest{RootID: r.ID, Version: r.SelectedVersion, Format: format, Name: "测试整合包", PackVersion: "1.2.3", Author: "Pack author", Path: filepath.Join(t.TempDir(), "export"+ext)}
}

func readExport(t *testing.T, name string) (*zip.ReadCloser, PackLock) {
	t.Helper()
	z, err := zip.OpenReader(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { z.Close() })
	lock, _, err := parsePack(z)
	if err != nil {
		t.Fatal(err)
	}
	return z, lock
}

func TestPackExportPreviewAndSelectedContents(t *testing.T) {
	s := testService(t)
	r := packExportFixture(t, s)
	preview, err := s.PreviewPackExport(context.Background(), r.ID, r.SelectedVersion)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Minecraft != "1.21.1" || preview.Loader != "fabric" || preview.LoaderVersion != "0.16.0" {
		t.Fatalf("wrong runtime: %+v", preview)
	}
	choices := map[string]bool{}
	for _, e := range preview.Entries {
		choices[e.Name] = e.Selected
	}
	want := map[string]bool{"mods": true, "config": true, "kubejs": true, "saves": false, "options.txt": false, "screenshots": false, "servers.dat": false}
	if !reflect.DeepEqual(choices, want) {
		t.Fatalf("private/generated data or wrong defaults: %#v", choices)
	}
	for _, names := range [][]string{nil, {"config", "saves"}, {}} {
		req := exportRequest(t, r, "curseforge") // offline, no credentials needed
		req.Entries = names
		id, err := s.ExportPack(req)
		if err != nil {
			t.Fatal(err)
		}
		job := waitJob(t, s, id)
		if job.State != "completed" || job.ResultRef != req.Path || job.TargetVersion != r.SelectedVersion {
			t.Fatalf("export failed: %+v", job)
		}
		z, lock := readExport(t, req.Path)
		if lock.Minecraft != preview.Minecraft || lock.Loader != preview.Loader || lock.LoaderVersion != preview.LoaderVersion {
			t.Fatalf("lost runtime: %+v", lock)
		}
		got := map[string]bool{}
		for _, f := range z.File {
			got[f.Name] = true
		}
		if !got["manifest.json"] || !got["overrides/"] || got["overrides/mods/other.jar"] || got["overrides/launcher_accounts.json"] || got["overrides/options.txt"] {
			t.Fatalf("wrong archive contents: %#v", got)
		}
		if names == nil && (!got["overrides/mods/local.jar"] || !got["overrides/mods/off.jar.disabled"] || !got["overrides/kubejs/assets/test/lang/en_us.json"] || !got["overrides/config/library/assets/file.bin"] || got["overrides/saves/World/level.dat"]) {
			t.Fatal("default export lost files or included a world")
		}
		if len(names) > 0 && (!got["overrides/saves/World/level.dat"] || got["overrides/mods/local.jar"]) {
			t.Fatal("explicit selection ignored")
		}
		if names != nil && len(names) == 0 && len(z.File) != 2 {
			t.Fatal("empty selection became default selection")
		}
	}
}

func TestPackExportModrinthReferencesAndRoundTrip(t *testing.T) {
	s := testService(t)
	r := packExportFixture(t, s)
	known, local := fabricJar(t, "known", "1"), []byte("changed locally")
	sha, sha512 := hashes(known)
	url := "https://cdn.modrinth.com/data/project/versions/version/known.jar"
	writeTestFile(t, filepath.Join(r.VersionDir(), "mods/known.jar"), known)
	writeTestFile(t, filepath.Join(r.VersionDir(), "mods/local.jar"), local)
	// A stale lock must not replace the user's modified bytes with old ones.
	if err := atomicJSON(filepath.Join(r.Path, ".mcl/pack-lock.json"), PackLock{GameVersion: r.SelectedVersion, Files: []PackFile{{Path: "mods/local.jar", SHA1: sha, SHA512: sha512, URLs: []string{url}}}}); err != nil {
		t.Fatal(err)
	}
	remote := map[string]any{sha: map[string]any{"files": []map[string]any{{"url": url, "size": len(known), "hashes": map[string]string{"sha1": sha, "sha512": sha512}}}}}
	fakeHTTP(t, s, map[string][]byte{modrinthAPI + "/version_files": jsonBytes(t, remote), url: known})
	req := exportRequest(t, r, "modrinth")
	id, err := s.ExportPack(req)
	if err != nil {
		t.Fatal(err)
	}
	if job := waitJob(t, s, id); job.State != "completed" {
		t.Fatal(job.Message)
	}
	z, lock := readExport(t, req.Path)
	if len(lock.Files) != 1 || lock.Files[0].Path != "mods/known.jar" || lock.Files[0].SHA512 != sha512 {
		t.Fatalf("wrong references: %+v", lock.Files)
	}
	if lock.Name != req.Name || lock.Version != req.PackVersion {
		t.Fatal("lost pack metadata")
	}
	for _, f := range z.File {
		if f.Name == "overrides/mods/known.jar" {
			t.Fatal("remote jar duplicated in overrides")
		}
	}
	// Exercise the same remote-file and override steps as ImportPack, byte for byte.
	dest := t.TempDir()
	if err := s.downloadPackFilesIdentified(context.Background(), dest, &lock, func(string, int, int) {}, nil); err != nil {
		t.Fatal(err)
	}
	if err := applyPackOverrides(context.Background(), z, dest, "overrides", &lock); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{"mods/known.jar": known, "mods/local.jar": local, "mods/off.jar.disabled": []byte("disabled mod"), "config/pack.toml": []byte("custom=true")} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("round trip changed %s: %v", name, err)
		}
	}
}

func TestPackExportCurseForgeIdentities(t *testing.T) {
	s := testService(t)
	s.publisher.CurseForgeAPIKey = "test-key"
	r := packExportFixture(t, s)
	known, found := []byte("known CF jar"), []byte("newly fingerprinted CF jar")
	ksha, _ := hashes(known)
	fsha, _ := hashes(found)
	writeTestFile(t, filepath.Join(r.VersionDir(), "mods/known.jar"), known)
	writeTestFile(t, filepath.Join(r.VersionDir(), "mods/found.jar"), found)
	writeTestFile(t, filepath.Join(r.VersionDir(), "mods/renamed.jar"), found)
	if err := atomicJSON(filepath.Join(r.VersionDir(), ".mcl/mods.json"), map[string]modRecord{
		"known.jar": {SHA1: ksha, Source: "curseforge", ProjectID: "11", VersionID: "22"},
		"local.jar": {SHA1: ksha, Source: "curseforge", ProjectID: "99", VersionID: "100"},
	}); err != nil {
		t.Fatal(err)
	}
	fakeHTTP(t, s, map[string][]byte{curseAPI + "/fingerprints": jsonBytes(t, map[string]any{"data": map[string]any{"exactMatches": []any{
		map[string]any{"file": map[string]any{"id": 44, "modId": 33, "fileName": "found.jar", "fileLength": len(found), "hashes": []any{map[string]any{"algo": 1, "value": fsha}}}},
	}}})})
	req := exportRequest(t, r, "curseforge")
	id, err := s.ExportPack(req)
	if err != nil {
		t.Fatal(err)
	}
	if job := waitJob(t, s, id); job.State != "completed" {
		t.Fatal(job.Message)
	}
	z, lock := readExport(t, req.Path)
	if len(lock.Files) != 2 {
		t.Fatalf("lost known or fingerprinted references: %+v", lock.Files)
	}
	for _, f := range lock.Files {
		if f.VersionID != "22" && f.VersionID != "44" {
			t.Fatal("stale identity exported")
		}
	}
	got := map[string]bool{}
	for _, f := range z.File {
		got[f.Name] = true
	}
	if !got["overrides/mods/renamed.jar"] || !got["overrides/mods/local.jar"] || !got["overrides/mods/off.jar.disabled"] {
		t.Fatal("renamed, modified or disabled file was replaced by a provider reference")
	}
}

func TestPackExportOfflineArchiveCanBeImported(t *testing.T) {
	for _, format := range []string{"modrinth", "curseforge"} {
		t.Run(format, func(t *testing.T) {
			s := testService(t)
			r := newTestRoot(t, s, "Vanilla pack")
			client := []byte("client")
			sha, _ := hashes(client)
			version := jsonBytes(t, gameVersion{ID: "1.21.1", MainClass: "net.minecraft.client.main.Main", Downloads: map[string]artifact{"client": {URL: "https://fixture.test/client", SHA1: sha}}})
			writeTestFile(t, filepath.Join(r.Path, "versions/1.21.1/1.21.1.json"), version)
			if err := s.Select(r.ID, "", "1.21.1"); err != nil {
				t.Fatal(err)
			}
			r, _ = s.getRoot(r.ID)
			writeTestFile(t, filepath.Join(r.VersionDir(), "config/test.txt"), []byte("a setting"))
			writeTestFile(t, filepath.Join(r.VersionDir(), "mods/local.jar.disabled"), fabricJar(t, "local", "1"))
			fakeHTTP(t, s, map[string][]byte{})
			req := exportRequest(t, r, format)
			id, err := s.ExportPack(req)
			if err != nil {
				t.Fatal(err)
			}
			if job := waitJob(t, s, id); job.State != "completed" {
				t.Fatal(job.Message)
			}
			vh, _ := hashes(version)
			fakeHTTP(t, s, map[string][]byte{manifestURL: jsonBytes(t, map[string]any{"versions": []map[string]string{{"id": "1.21.1", "url": "https://fixture.test/version", "sha1": vh}}}), "https://fixture.test/version": version, "https://fixture.test/client": client})
			id, err = s.ImportPack(ImportRequest{Path: req.Path, Parent: t.TempDir(), Name: "Reimported"})
			if err != nil {
				t.Fatal(err)
			}
			job := waitJob(t, s, id)
			if job.State != "completed" {
				t.Fatal(job.Message)
			}
			var imported Root
			for _, candidate := range s.Snapshot().Instances {
				if candidate.ID == job.ResultRoot {
					imported = candidate.Root
				}
			}
			if imported.ID == "" {
				t.Fatalf("missing import result: %+v", job)
			}
			b, err := os.ReadFile(filepath.Join(imported.VersionDir(), "config/test.txt"))
			if err != nil || string(b) != "a setting" {
				t.Fatalf("settings failed round trip: %q %v", b, err)
			}
			if _, err := os.Stat(filepath.Join(imported.VersionDir(), "mods/local.jar.disabled")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPackExportCancellationAndFailurePreserveDestination(t *testing.T) {
	s := testService(t)
	r := packExportFixture(t, s)
	for _, cancelStage := range []string{"scan", "write", "replace"} {
		t.Run(cancelStage, func(t *testing.T) {
			req := exportRequest(t, r, "curseforge")
			req.Entries = []string{"config", "kubejs"}
			req.Overwrite = true
			writeTestFile(t, req.Path, []byte("keep old export"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := s.exportPack(ctx, r, req, func(msg string, _, _ int) {
				if cancelStage == "scan" && msg == "正在整理导出内容" || cancelStage == "write" && strings.HasPrefix(msg, "正在导出：") {
					cancel()
				}
				if cancelStage == "replace" && strings.HasPrefix(msg, "导出文件引用") {
					writeTestFile(t, req.Path, []byte("concurrently changed"))
				}
			})
			if err == nil || cancelStage != "replace" && !errors.Is(err, context.Canceled) {
				t.Fatalf("export should abort: %v", err)
			}
			b, _ := os.ReadFile(req.Path)
			want := "keep old export"
			if cancelStage == "replace" {
				want = "concurrently changed"
			}
			if string(b) != want {
				t.Fatalf("existing export overwritten: %q", b)
			}
			entries, _ := os.ReadDir(filepath.Dir(req.Path))
			if len(entries) != 1 {
				t.Fatal("temporary export leaked")
			}
		})
	}
	// Failed writes are also cleaned, even though failed jobs remain retryable.
	req := exportRequest(t, r, "curseforge")
	req.Entries = []string{"config", "kubejs"}
	err := s.exportPack(context.Background(), r, req, func(msg string, _, _ int) {
		if strings.HasPrefix(msg, "正在导出：config/") {
			writeTestFile(t, filepath.Join(r.VersionDir(), "kubejs/server_scripts/example.js"), []byte("changed after scan"))
		}
	})
	if err == nil {
		t.Fatal("changed file accepted")
	}
	entries, _ := os.ReadDir(filepath.Dir(req.Path))
	if len(entries) != 0 {
		t.Fatal("failed export left a partial archive")
	}
}

func TestPackExportRejectsUnsafeSelectionsAndBusyRoots(t *testing.T) {
	s := testService(t)
	r := packExportFixture(t, s)
	for _, entries := range [][]string{{"../outside"}, {"/absolute"}, {"mods/../../outside"}, {".mcl"}, {"logs"}, {"launcher_accounts.json"}} {
		req := exportRequest(t, r, "curseforge")
		req.Entries = entries
		if err := s.exportPack(context.Background(), r, req, func(string, int, int) {}); err == nil {
			t.Fatalf("unsafe selection accepted: %v", entries)
		}
	}
	req := exportRequest(t, r, "curseforge")
	req.Path = filepath.Join(r.VersionDir(), "recursive.zip")
	if err := s.exportPack(context.Background(), r, req, func(string, int, int) {}); err == nil {
		t.Fatal("export inside its input accepted")
	}
	req = exportRequest(t, r, "curseforge")
	s.mu.Lock()
	s.rootLocks[r.ID] = true
	s.mu.Unlock()
	if _, err := s.ExportPack(req); err == nil {
		t.Fatal("busy root accepted")
	}
	s.mu.Lock()
	delete(s.rootLocks, r.ID)
	s.mu.Unlock()
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), []byte("secret"))
	if err := scanTestDirectoryLink(outside, filepath.Join(r.VersionDir(), "config/linked")); err != nil {
		t.Skipf("directory links unavailable: %v", err)
	}
	req.Entries = []string{"config"}
	if err := s.exportPack(context.Background(), r, req, func(string, int, int) {}); err == nil {
		t.Fatal("linked content escaped the instance")
	}
}

func TestPackExportProviderFailureIsBoundedAndFallsBack(t *testing.T) {
	s := testService(t)
	r := packExportFixture(t, s)
	calls := 0
	s.http = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unavailable")), Request: req}, nil
	})}
	for i := 0; i < 130; i++ {
		writeTestFile(t, filepath.Join(r.VersionDir(), fmt.Sprintf("mods/m%03d.jar", i)), []byte(fmt.Sprint(i)))
	}
	req := exportRequest(t, r, "modrinth")
	if err := s.exportPack(context.Background(), r, req, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("continued failed lookups: %d", calls)
	}
	z, lock := readExport(t, req.Path)
	if len(lock.Files) != 0 || len(z.File) < 130 {
		t.Fatal("offline export omitted local content")
	}
}

func TestPackExportLoaderManifests(t *testing.T) {
	for _, format := range []string{"modrinth", "curseforge"} {
		for _, loader := range []string{"vanilla", "forge", "neoforge", "fabric", "quilt"} {
			t.Run(format+"/"+loader, func(t *testing.T) {
				req := ExportPackRequest{Format: format, Name: "Pack", PackVersion: "1", Author: "Author"}
				preview := PackExportPreview{Minecraft: "1.21.1", Loader: loader, LoaderVersion: "1.2.3"}
				if loader == "vanilla" {
					preview.LoaderVersion = ""
				}
				var b bytes.Buffer
				zw := zip.NewWriter(&b)
				if err := writePackExportIndex(context.Background(), zw, req, preview, nil); err != nil {
					t.Fatal(err)
				}
				if err := zw.Close(); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(t.TempDir(), "manifest.zip")
				writeTestFile(t, file, b.Bytes())
				z, lock := readExport(t, file)
				if lock.Loader != loader || lock.LoaderVersion != preview.LoaderVersion {
					t.Fatalf("wrong loader: %+v", lock)
				}
				rd, err := z.File[0].Open()
				if err != nil {
					t.Fatal(err)
				}
				defer rd.Close()
				var raw map[string]any
				if err := json.NewDecoder(rd).Decode(&raw); err != nil {
					t.Fatal(err)
				}
				if _, ok := raw["files"].([]any); !ok {
					t.Fatal("empty manifest files must be an array")
				}
			})
		}
	}
}

func BenchmarkPackExportArchiveMemory(b *testing.B) {
	for _, size := range []int64{1 << 20, 64 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			base := b.TempDir()
			f, err := os.Create(filepath.Join(base, "local.jar"))
			if err != nil {
				b.Fatal(err)
			}
			if err := f.Truncate(size); err != nil {
				b.Fatal(err)
			}
			st, err := f.Stat()
			if err != nil {
				b.Fatal(err)
			}
			f.Close()
			dir, err := os.OpenRoot(base)
			if err != nil {
				b.Fatal(err)
			}
			defer dir.Close()
			files := []packExportFile{{path: "local.jar", size: size, mtime: st.ModTime(), mode: st.Mode()}}
			req := ExportPackRequest{Format: "curseforge", Name: "Pack", PackVersion: "1"}
			preview := PackExportPreview{Minecraft: "1.21.1", Loader: "vanilla"}
			b.ReportAllocs()
			b.SetBytes(size)
			b.ResetTimer()
			for range b.N {
				if err := writePackExportArchive(context.Background(), dir, io.Discard, req, preview, files, func(string, int, int) {}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
