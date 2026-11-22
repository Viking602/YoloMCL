package launcher

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func undoFixture(t *testing.T) (*Service, Root, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".minecraft")
	for _, v := range []string{"v1", "v2"} {
		dir := filepath.Join(path, "versions", v, "mods")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), v+".json"), []byte(`{"id":"`+v+`","mainClass":"Main"}`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "example.jar"), []byte(v), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AddRoot(path, "Saved pack")
	if err != nil {
		t.Fatal(err)
	}
	return s, r, path
}

func TestModUndoUsesOriginalVersionAndRefusesOverwrite(t *testing.T) {
	s, r, path := undoFixture(t)
	if err := s.Select(r.ID, "", "v2"); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.RemoveModUndo(r.ID, "v1", "example.jar")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(path, "versions", "v1", "mods", "example.jar")
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("original mod was not removed")
	}
	other, err := os.ReadFile(filepath.Join(path, "versions", "v2", "mods", "example.jar"))
	if err != nil || string(other) != "v2" {
		t.Fatal("selected version was changed")
	}
	if err := os.WriteFile(source, []byte("new mod"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreMod(receipt); err == nil {
		t.Fatal("undo overwrote a new mod")
	}
	got, _ := os.ReadFile(source)
	if string(got) != "new mod" {
		t.Fatal("new contents lost")
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreMod(receipt); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(source)
	if string(got) != "v1" {
		t.Fatal("undo restored another version")
	}
	if err := s.RestoreMod(receipt); err == nil {
		t.Fatal("receipt reused")
	}
}

func TestRootUndoPreservesOverridesAndPersistence(t *testing.T) {
	s, r, _ := undoFixture(t)
	if err := s.UpdateRoot(r.ID, "Custom", "", 4096, "-Droot=true"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVersionConfig(r.ID, "v1", "", 2048, "-Dversion=true"); err != nil {
		t.Fatal(err)
	}
	before, err := s.getRoot(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.RemoveRootUndo(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getRoot(r.ID); err == nil {
		t.Fatal("root still registered")
	}
	if err := s.RestoreRoot(receipt); err != nil {
		t.Fatal(err)
	}
	after, err := s.getRoot(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("undo lost configuration: before=%+v after=%+v", before, after)
	}
	if err := s.RestoreRoot(receipt); err == nil {
		t.Fatal("repeated undo replaced current config")
	}
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()
	reopened, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.getRoot(r.ID)
	if err != nil || !reflect.DeepEqual(after, persisted) {
		t.Fatal("restored configuration was not persisted", err)
	}
}

func TestAssetAndAuthServerUndo(t *testing.T) {
	s, r, path := undoFixture(t)
	for _, item := range []struct{ kind, folder string }{{"resourcepack", "resourcepacks"}, {"shader", "shaderpacks"}} {
		folder := filepath.Join(path, "versions", "v1", item.folder)
		if err := os.MkdirAll(folder, 0755); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(folder, "example.zip")
		if err := os.WriteFile(file, []byte("original resource"), 0644); err != nil {
			t.Fatal(err)
		}
		receipt, err := s.RemoveAssetPackUndo(r.ID, "v1", item.kind, "example.zip")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("resource not removed")
		}
		if err := s.RestoreMod(receipt); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(file)
		if err != nil || string(b) != "original resource" {
			t.Fatal("resource contents lost")
		}
	}
	server, err := s.SaveAuthServer("", "Saved server", "https://auth.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAuthServer(server.URL); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreAuthServer(server); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveAuthServer(server.URL, "New server configuration", server.URL); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreAuthServer(server); err == nil {
		t.Fatal("undo overwrote a new server configuration")
	}
	for _, v := range s.Snapshot().AuthServers {
		if SameAuthServer(v.URL, server.URL) && v.Name != "New server configuration" {
			t.Fatal("new server configuration lost")
		}
	}
}

func TestHorizonModAndShaderRemovalUndo(t *testing.T) {
	s, r, path := undoFixture(t)
	hVer := "horizon-nightly-2026-11-20-0a875da"
	hDir := filepath.Join(path, "versions", hVer)
	writeTestFile(t, filepath.Join(hDir, hVer+".json"), jsonBytes(t, map[string]any{
		"id":            hVer,
		"clientVersion": "26.3",
		"type":          "release",
		"horizon": map[string]any{
			"buildId":   "nightly-2026-11-20-0a875da",
			"minecraft": "26.3",
		},
	}))

	// 1. Horizon .hmod file with horizon-mods.txt state and .mcl/mods.json record.
	hmodName := "crate_delight-26.07.01-horizon.3.hmod"
	hmodPath := filepath.Join(hDir, "horizon-mods", hmodName)
	writeTestFile(t, hmodPath, zipBytes(t, map[string][]byte{
		"horizon-mod.toml": []byte("id = \"crate_delight\"\nname = \"Crate Delight\"\nversion = \"26.07.01-horizon.3\"\n"),
	}))
	writeTestFile(t, filepath.Join(hDir, "horizon-mods.txt"), []byte("mod.crate_delight:true\nmod.other:false\n"))
	writeTestFile(t, filepath.Join(hDir, ".mcl", "mods.json"), jsonBytes(t, map[string]modRecord{
		hmodName: {SHA1: "abc123", Source: "horizon", ProjectID: "crate_delight", VersionID: "26.07.01-horizon.3"},
	}))

	// Keep selected version on v2 to verify explicit version parameter selects Horizon properly.
	if err := s.Select(r.ID, "", "v2"); err != nil {
		t.Fatal(err)
	}

	receipt, err := s.RemoveModUndo(r.ID, hVer, hmodName)
	if err != nil {
		t.Fatalf("RemoveModUndo(.hmod): %v", err)
	}
	if _, err := os.Stat(hmodPath); !os.IsNotExist(err) {
		t.Fatal(".hmod was not removed")
	}
	enabledAfterRemove, err := readHorizonEnabledMods(t.Context(), hDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := enabledAfterRemove["crate_delight"]; ok {
		t.Fatalf("crate_delight state not removed from horizon-mods.txt: %+v", enabledAfterRemove)
	}
	if val, ok := enabledAfterRemove["other"]; !ok || val {
		t.Fatalf("unrelated entry in horizon-mods.txt corrupted: %+v", enabledAfterRemove)
	}
	if _, ok := readRecords(hDir)[hmodName]; ok {
		t.Fatal("mods.json record not removed")
	}

	if err := s.RestoreMod(receipt); err != nil {
		t.Fatalf("RestoreMod(.hmod): %v", err)
	}
	if _, err := os.Stat(hmodPath); err != nil {
		t.Fatalf(".hmod not restored: %v", err)
	}
	enabledAfterRestore, err := readHorizonEnabledMods(t.Context(), hDir)
	if err != nil {
		t.Fatal(err)
	}
	if !enabledAfterRestore["crate_delight"] {
		t.Fatalf("crate_delight enabled state not restored: %+v", enabledAfterRestore)
	}
	if rec, ok := readRecords(hDir)[hmodName]; !ok || rec.ProjectID != "crate_delight" {
		t.Fatalf("mods.json record not restored: %+v", readRecords(hDir))
	}

	// 2. Unpacked Horizon mod directory with horizon-mod.toml.
	dirMod := filepath.Join(hDir, "horizon-mods", "unpacked_mod")
	writeTestFile(t, filepath.Join(dirMod, "horizon-mod.toml"), []byte("id = \"unpacked_mod\"\nname = \"Unpacked\"\n"))
	dirReceipt, err := s.RemoveModUndo(r.ID, hVer, "unpacked_mod")
	if err != nil {
		t.Fatalf("RemoveModUndo(dir mod): %v", err)
	}
	if _, err := os.Stat(dirMod); !os.IsNotExist(err) {
		t.Fatal("unpacked mod dir not removed")
	}
	if err := s.RestoreMod(dirReceipt); err != nil {
		t.Fatalf("RestoreMod(dir mod): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirMod, "horizon-mod.toml")); err != nil {
		t.Fatalf("unpacked mod dir not restored: %v", err)
	}

	// 3. Invalid mod names are still rejected for both Horizon and Java versions.
	writeTestFile(t, filepath.Join(hDir, "horizon-mods", "notes.txt"), []byte("not a mod"))
	if _, err := s.RemoveModUndo(r.ID, hVer, "notes.txt"); err == nil {
		t.Fatal("accepted non-.hmod file in horizon-mods")
	}
	if _, err := s.RemoveModUndo(r.ID, "v1", hmodName); err == nil {
		t.Fatal("accepted .hmod on non-Horizon Java version")
	}

	// 4. Uppercase .JAR Java mod removal and undo.
	upperJar := filepath.Join(path, "versions", "v1", "mods", "UpperMod.JAR")
	writeTestFile(t, upperJar, []byte("jar"))
	upperReceipt, err := s.RemoveModUndo(r.ID, "v1", "UpperMod.JAR")
	if err != nil {
		t.Fatalf("RemoveModUndo(.JAR): %v", err)
	}
	if err := s.RestoreMod(upperReceipt); err != nil {
		t.Fatalf("RestoreMod(.JAR): %v", err)
	}

	// 5. Horizon .hshader file and folder packs (pack.mcmeta / horizon-shaders.toml).
	hshader := filepath.Join(hDir, "shaderpacks", "aurora.hshader")
	writeTestFile(t, hshader, zipBytes(t, map[string][]byte{"horizon-shaders.toml": []byte("description = \"Aurora\"\n")}))
	shaderReceipt, err := s.RemoveAssetPackUndo(r.ID, hVer, "shader", "aurora.hshader")
	if err != nil {
		t.Fatalf("RemoveAssetPackUndo(.hshader): %v", err)
	}
	if _, err := os.Stat(hshader); !os.IsNotExist(err) {
		t.Fatal(".hshader not removed")
	}
	if err := s.RestoreMod(shaderReceipt); err != nil {
		t.Fatalf("RestoreMod(.hshader): %v", err)
	}
	if _, err := s.RemoveAssetPackUndo(r.ID, hVer, "resourcepack", "aurora.hshader"); err == nil {
		t.Fatal("accepted .hshader as resourcepack")
	}

	folderRP := filepath.Join(hDir, "resourcepacks", "MyFolderPack")
	writeTestFile(t, filepath.Join(folderRP, "pack.mcmeta"), []byte(`{"pack":{"description":"Folder"}}`))
	rpReceipt, err := s.RemoveAssetPackUndo(r.ID, hVer, "resourcepack", "MyFolderPack")
	if err != nil {
		t.Fatalf("RemoveAssetPackUndo(folder resourcepack): %v", err)
	}
	if err := s.RestoreMod(rpReceipt); err != nil {
		t.Fatalf("RestoreMod(folder resourcepack): %v", err)
	}

	folderShader := filepath.Join(hDir, "shaderpacks", "MyFolderShader")
	writeTestFile(t, filepath.Join(folderShader, "horizon-shaders.toml"), []byte("description = \"Folder shader\"\n"))
	fShaderReceipt, err := s.RemoveAssetPackUndo(r.ID, hVer, "shader", "MyFolderShader")
	if err != nil {
		t.Fatalf("RemoveAssetPackUndo(folder shader): %v", err)
	}
	if err := s.RestoreMod(fShaderReceipt); err != nil {
		t.Fatalf("RestoreMod(folder shader): %v", err)
	}
}
