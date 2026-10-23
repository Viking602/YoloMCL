package launcher

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nbtLevel builds a tiny level.dat: root{Data{LevelName,GameType,LastPlayed,hardcore,+noise}}.
func nbtLevel(t *testing.T, name string, mode int32, last int64) []byte {
	var b bytes.Buffer
	str := func(s string) { _ = binary.Write(&b, binary.BigEndian, uint16(len(s))); b.WriteString(s) }
	b.WriteByte(10)
	str("")
	b.WriteByte(10)
	str("Data")
	b.WriteByte(9) // list noise before the fields
	str("Players")
	b.WriteByte(3)
	_ = binary.Write(&b, binary.BigEndian, int32(2))
	_ = binary.Write(&b, binary.BigEndian, int64(0))
	b.WriteByte(10)
	str("Player")
	b.WriteByte(8)
	str("LevelName") // same name, wrong path: must be ignored
	str("WRONG")
	b.WriteByte(0)
	b.WriteByte(8)
	str("LevelName")
	str(name)
	b.WriteByte(3)
	str("GameType")
	_ = binary.Write(&b, binary.BigEndian, mode)
	b.WriteByte(4)
	str("LastPlayed")
	_ = binary.Write(&b, binary.BigEndian, last)
	b.WriteByte(1)
	str("hardcore")
	b.WriteByte(0)
	b.WriteByte(0)
	b.WriteByte(0)
	var z bytes.Buffer
	w := gzip.NewWriter(&z)
	w.Write(b.Bytes())
	w.Close()
	return z.Bytes()
}

func TestParseLevelDat(t *testing.T) {
	li, e := ParseLevelDat(bytes.NewReader(nbtLevel(t, "我的世界 1", 1, 1761100000000)))
	if e != nil || li.Name != "我的世界 1" || li.GameType != 1 || li.LastPlayed != 1761100000000 || li.Hardcore {
		t.Fatalf("%+v %v", li, e)
	}
	if _, e := ParseLevelDat(bytes.NewReader([]byte("junk"))); e == nil {
		t.Fatal("junk accepted")
	}
}

func worldFixture(t *testing.T) (*Service, Root, string) {
	s := testService(t)
	dir := t.TempDir()
	r, e := s.createEmptyRoot(dir, "W")
	if e != nil {
		t.Fatal(e)
	}
	w := filepath.Join(r.Path, "saves", "w1")
	writeTestFile(t, filepath.Join(w, "level.dat"), nbtLevel(t, "World One", 0, 1))
	writeTestFile(t, filepath.Join(w, "region", "r.0.0.mca"), []byte("blocks-v1"))
	writeTestFile(t, filepath.Join(w, "session.lock"), []byte("lock"))
	return s, r, w
}

func TestWorldBackupRoundTripAndRestore(t *testing.T) {
	s, r, w := worldFixture(t)
	ws, e := s.ListWorlds(r.ID, "")
	if e != nil || len(ws) != 1 || ws[0].Name != "World One" || ws[0].Mode != "survival" {
		t.Fatalf("%+v %v", ws, e)
	}
	id, e := s.BackupWorld(r.ID, "", "w1")
	if e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, s, id); j.State != "completed" {
		t.Fatalf("%+v", j)
	}
	bs, _ := s.WorldBackups(r.ID, "", "w1")
	if len(bs) != 1 || bs[0].Auto || !strings.HasPrefix(bs[0].File, "w1-") {
		t.Fatalf("%+v", bs)
	}
	// change the world, then restore
	writeTestFile(t, filepath.Join(w, "region", "r.0.0.mca"), []byte("griefed"))
	writeTestFile(t, filepath.Join(w, "new.dat"), []byte("x"))
	id, e = s.RestoreWorld(r.ID, "", "w1", bs[0].File)
	if e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, s, id); j.State != "completed" {
		t.Fatalf("%+v", j)
	}
	if b, _ := os.ReadFile(filepath.Join(w, "region", "r.0.0.mca")); string(b) != "blocks-v1" {
		t.Fatalf("restored %q", b)
	}
	if _, e := os.Stat(filepath.Join(w, "new.dat")); !os.IsNotExist(e) {
		t.Fatal("restore merged instead of replacing")
	}
	if _, e := os.Stat(filepath.Join(w, "session.lock")); !os.IsNotExist(e) {
		t.Fatal("session.lock was backed up")
	}
	bs, _ = s.WorldBackups(r.ID, "", "w1")
	if len(bs) != 2 || !bs[0].Auto && !bs[1].Auto {
		t.Fatalf("no auto backup: %+v", bs)
	}
	ents, _ := os.ReadDir(filepath.Join(r.Path, "saves"))
	if len(ents) != 1 {
		t.Fatalf("temp left in saves: %v", ents)
	}
}

// A broken backup never touches the current world.
func TestWorldRestoreIsAtomic(t *testing.T) {
	s, r, w := worldFixture(t)
	bad := filepath.Join(r.Path, "backups", "w1", "w1-20260101-000000.zip")
	writeTestFile(t, bad, zipBytes(t, map[string][]byte{"w1/region/x": []byte("no level.dat")}))
	id, e := s.RestoreWorld(r.ID, "", "w1", filepath.Base(bad))
	if e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, s, id); j.State != "failed" {
		t.Fatalf("%+v", j)
	}
	if b, _ := os.ReadFile(filepath.Join(w, "region", "r.0.0.mca")); string(b) != "blocks-v1" {
		t.Fatal("world changed by failed restore")
	}
	_ = s.RemoveJob(id)
	ents, _ := os.ReadDir(filepath.Join(r.Path, "saves"))
	if len(ents) != 1 {
		t.Fatalf("temp left: %v", ents)
	}
	_ = context.Background
}
