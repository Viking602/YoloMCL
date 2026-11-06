package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenFingerprintTracksInputs(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	os.MkdirAll(cfg, 0o700)
	mod := filepath.Join(dir, "a.jar")
	os.WriteFile(mod, []byte("x"), 0o600)
	os.WriteFile(filepath.Join(cfg, "m.toml"), []byte("a=1"), 0o600)
	in := WorldgenInput{Minecraft: "1.21.1", Sources: []WorldgenSource{{mod, "mod:a.jar"}}}
	fp := func() string {
		k, err := worldgenOpenFingerprint(context.Background(), in, []string{cfg, filepath.Join(dir, "missing")})
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	base := fp()
	if fp() != base {
		t.Fatal("fingerprint not stable")
	}
	os.WriteFile(filepath.Join(cfg, "m.toml"), []byte("a=22"), 0o600)
	k2 := fp()
	if k2 == base {
		t.Fatal("config edit not detected")
	}
	later := time.Now().Add(time.Hour)
	os.Chtimes(mod, later, later)
	if fp() == k2 {
		t.Fatal("jar change not detected")
	}
	in.EnabledPacks = []string{"file/x"}
	if fp() == k2 {
		t.Fatal("input change not detected")
	}
}
