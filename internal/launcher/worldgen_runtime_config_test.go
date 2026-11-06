package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorldgenRequestedConfigUsesSavedServerOverride(t *testing.T) {
	root := t.TempDir()
	for path, value := range map[string]string{"config/common.toml": "common", "defaultconfigs/example.toml": "default", "saves/world/serverconfig/example.toml": "saved"} {
		p := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct{ world, name, want string }{{"", "common.toml", "common"}, {"world", "common.toml", "common"}, {"", "serverconfig/example.toml", "default"}, {"world", "serverconfig/example.toml", "saved"}, {"absent", "serverconfig/example.toml", "default"}, {"world", "serverconfig/missing.toml", ""}} {
		got, err := readSeedMapModConfig(context.Background(), root, tt.world, tt.name)
		if err != nil || string(got) != tt.want {
			t.Fatalf("%+v: %q %v", tt, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readSeedMapModConfig(ctx, root, "world", "serverconfig/example.toml"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := readSeedMapModConfig(context.Background(), root, "../other", "serverconfig/example.toml"); err == nil {
		t.Fatal("invalid world directory")
	}
}
