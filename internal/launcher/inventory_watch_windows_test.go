package launcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsInventoryNotifications(t *testing.T) {
	for _, operation := range []string{"add", "rename", "edit", "delete"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "versions", "pack", "mods", "one.jar")
			writeTestFile(t, file, []byte("one"))
			watch := watchInventoryChanges(root)
			if watch == nil {
				t.Fatal("local test directory has no watcher")
			}
			defer watch.close()
			if changed, err := watch.changed(); changed || err != nil {
				t.Fatalf("unchanged directory signalled: %v, %v", changed, err)
			}
			var err error
			switch operation {
			case "add":
				err = os.WriteFile(filepath.Join(filepath.Dir(file), "two.jar"), []byte("two"), 0600)
			case "rename":
				err = os.Rename(file, file+".disabled")
			case "edit":
				err = os.WriteFile(file, []byte("changed"), 0600)
			case "delete":
				err = os.Remove(file)
			}
			if err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
				changed, err := watch.changed()
				if err != nil {
					t.Fatal(err)
				}
				if changed {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("nested file change did not signal")
				}
			}
		})
	}
}

func TestWindowsInventoryWatchDetectsRootReplacementBeforeFirstPoll(t *testing.T) {
	base := t.TempDir()
	root, moved := filepath.Join(base, "root"), filepath.Join(base, "moved")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	watch := watchInventoryChanges(root)
	if watch == nil {
		t.Fatal("local test directory has no watcher")
	}
	defer watch.close()
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if changed, err := watch.changed(); !changed || err == nil {
		t.Fatal("replaced root was mistaken for the original directory")
	}
}
