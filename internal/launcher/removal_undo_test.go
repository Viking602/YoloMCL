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
