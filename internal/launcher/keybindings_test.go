package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

const bindingFixtureOptions = "\ufeffversion:3955\r\nkey_key.forward:key.keyboard.w\r\nkey_quest.open:key.keyboard.o:CONTROL\r\nkey_key.fakeModAction:key.keyboard.p\r\nsoundCategory_music:0.2\r\nresourcePacks:[\"vanilla\",\"file/a:b.zip\"]\r\nunknownOption:keep:every:colon"

func bindingFixture(t *testing.T, s *Service) Root {
	t.Helper()
	r := newTestRoot(t, s, "keybinding-pack")
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(bindingFixtureOptions))
	return r
}

func saveBindings(t *testing.T, s *Service, root string, bindings map[string]string) KeyBindingsView {
	t.Helper()
	v, err := s.ReadKeyBindings(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.SaveKeyBindings(context.Background(), KeyBindingsRequest{RootID: root, Revision: v.Revision, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func applyBindings(t *testing.T, s *Service, root string) KeyBindingsView {
	t.Helper()
	v, err := s.ReadKeyBindings(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.ApplyKeyBindings(context.Background(), KeyBindingsRequest{RootID: root, Revision: v.Revision})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestKeyBindingsInheritanceIsolationAndRestore(t *testing.T) {
	s := testService(t)
	a, b := bindingFixture(t, s), bindingFixture(t, s)
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i"})
	saveBindings(t, s, a.ID, map[string]string{"key_key.forward": "key.keyboard.k", "key_quest.open": "key.keyboard.unknown"})
	applyBindings(t, s, a.ID)
	applyBindings(t, s, b.ID)
	av := parseKeyBindings([]byte(zzr(filepath.Join(a.Path, "options.txt"))))
	bv := parseKeyBindings([]byte(zzr(filepath.Join(b.Path, "options.txt"))))
	if av["key_key.forward"] != "key.keyboard.k" || av["key_quest.open"] != "key.keyboard.unknown" || bv["key_key.forward"] != "key.keyboard.i" || bv["key_quest.open"] != "key.keyboard.o:CONTROL" {
		t.Fatalf("wrong inheritance or cross-pack modification: %v / %v", av, bv)
	}
	saveBindings(t, s, a.ID, nil)
	applyBindings(t, s, a.ID)
	av = parseKeyBindings([]byte(zzr(filepath.Join(a.Path, "options.txt"))))
	if av["key_key.forward"] != "key.keyboard.i" || av["key_quest.open"] != "key.keyboard.o:CONTROL" {
		t.Fatal("restoring inheritance lost the game/mod baseline", av)
	}
	saveBindings(t, s, "", nil)
	applyBindings(t, s, a.ID)
	if got := zzr(filepath.Join(a.Path, "options.txt")); got != bindingFixtureOptions {
		t.Fatalf("unmanaged contents, BOM, CRLF or original keys changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(a.Path, ".mcl", "keybindings-options.bak")); err != nil {
		t.Fatal("missing independent options backup", err)
	}
}

func TestKeyBindingsRejectNonVanillaAndStaleChanges(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	ctx := context.Background()
	global, _ := s.ReadKeyBindings(ctx, "", "")
	for _, key := range []string{"key_quest.open", "key_key.fakeModAction", "fullscreen"} {
		if _, err := s.SaveKeyBindings(ctx, KeyBindingsRequest{Revision: global.Revision, Bindings: map[string]string{key: "key.keyboard.a"}}); err == nil {
			t.Fatal("accepted non-vanilla global key", key)
		}
	}
	saveBindings(t, s, "", map[string]string{"key_key.jump": "key.keyboard.space"})
	if _, err := s.SaveKeyBindings(ctx, KeyBindingsRequest{Revision: global.Revision, Bindings: nil}); err == nil {
		t.Fatal("stale global draft overwrote newer bindings")
	}
	view, _ := s.ReadKeyBindings(ctx, r.ID, "")
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(bindingFixtureOptions+"\nnewOption:game-edit"))
	request := KeyBindingsRequest{RootID: r.ID, Revision: view.Revision, Bindings: map[string]string{"key_key.forward": "key.keyboard.i"}}
	if _, err := s.SaveKeyBindings(ctx, request); err == nil {
		t.Fatal("stale options draft saved")
	}
	if _, err := s.ApplyKeyBindings(ctx, request); err == nil {
		t.Fatal("stale options plan applied")
	}
	view, _ = s.ReadKeyBindings(ctx, r.ID, "")
	request.Revision, request.Bindings = view.Revision, map[string]string{"key_absent.mod": "key.keyboard.a"}
	if _, err := s.SaveKeyBindings(ctx, request); err == nil {
		t.Fatal("invented a key absent from options")
	}
	s.rootLocks[r.ID] = true
	if _, err := s.ApplyKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Revision: view.Revision}); err == nil {
		t.Fatal("applied during another root operation")
	}
	delete(s.rootLocks, r.ID)
	s.running[runKey(r.ID, "test")] = &runningGame{}
	if _, err := s.SaveKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Revision: view.Revision}); err == nil {
		t.Fatal("saved pack policy while game was running")
	}
	if _, err := s.ApplyKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Revision: view.Revision}); err == nil {
		t.Fatal("modified live game options")
	}
	delete(s.running, runKey(r.ID, "test"))
}

func TestKeyBindingsLegacyAndModifiers(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte("key_key.forward:17\nkey_key.swapHands:33\nkey_quest.open:24:CONTROL\nkey_key.attack:-100\n"))
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i", "key_key.swapOffhand": "key.keyboard.h", "key_key.attack": "key.mouse.right"})
	saveBindings(t, s, r.ID, map[string]string{"key_quest.open": "key.keyboard.k:SHIFT"})
	applyBindings(t, s, r.ID)
	got := zzr(filepath.Join(r.Path, "options.txt"))
	if got != "key_key.forward:23\nkey_key.swapHands:35\nkey_quest.open:37:SHIFT\nkey_key.attack:-99\n" {
		t.Fatalf("wrong legacy encoding or alias: %q", got)
	}
	saveBindings(t, s, r.ID, map[string]string{"key_quest.open": "key.keyboard.unknown"})
	applyBindings(t, s, r.ID)
	if !strings.Contains(zzr(filepath.Join(r.Path, "options.txt")), "key_quest.open:0\n") {
		t.Fatal("legacy unbound value is not 0")
	}
	for _, input := range []string{"key.keyboard.w\nfullscreen:true", "key.keyboard.a:INVALID", "", "17:SHIFT:CONTROL"} {
		if _, err := NormalizeKeyBinding(input); err == nil {
			t.Fatal("invalid binding accepted", input)
		}
	}
}

func TestKeyBindingsPersistenceAndGameEdits(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i"})
	applyBindings(t, s, r.ID)
	reopened, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	view, err := reopened.ReadKeyBindings(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range view.Rows {
		found = found || row.ID == "key_key.forward" && row.Override == "key.keyboard.i"
	}
	if !found {
		t.Fatal("global bindings lost on restart")
	}
	// Releasing management must preserve a later edit made by the game.
	path := filepath.Join(r.Path, "options.txt")
	writeTestFile(t, path, []byte(strings.ReplaceAll(zzr(path), "key.keyboard.i", "key.keyboard.u")))
	saveBindings(t, reopened, "", nil)
	applyBindings(t, reopened, r.ID)
	if parseKeyBindings([]byte(zzr(path)))["key_key.forward"] != "key.keyboard.u" {
		t.Fatal("unmanagement destroyed a newer game edit")
	}
	cloned := s.cfg.clone()
	cloned.KeyBindings["key_key.forward"] = "key.keyboard.p"
	if s.globalKeyBindings()["key_key.forward"] != "key.keyboard.i" {
		t.Fatal("configuration clone shared the mutable global map")
	}
}

func TestKeyBindingsRecoverInterruptedApply(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-options", true: "after-options"}[after], func(t *testing.T) {
			s := testService(t)
			r := bindingFixture(t, s)
			saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.h"})
			before := []byte("key_key.forward:key.keyboard.i\n")
			afterData := []byte("key_key.forward:key.keyboard.h\n")
			f, _ := bindingFiles(r)
			state := keyBindingState{Schema: 1, Keys: map[string]appliedKeyBinding{"key_key.forward": {Original: "key.keyboard.w", Applied: "key.keyboard.i"}}, Pending: &keyBindingPending{Before: bindingRevision(nil, nil, before), After: bindingRevision(nil, nil, afterData), Keys: map[string]appliedKeyBinding{"key_key.forward": {Original: "key.keyboard.w", Applied: "key.keyboard.h"}}}}
			if err := atomicJSON(f.state, state); err != nil {
				t.Fatal(err)
			}
			data := before
			if after {
				data = afterData
			}
			writeTestFile(t, f.options, data)
			applyBindings(t, s, r.ID)
			clean, err := readBindingState(context.Background(), f.state)
			if err != nil || clean.Pending != nil {
				t.Fatal("journal was not finalized", err)
			}
			saveBindings(t, s, "", nil)
			applyBindings(t, s, r.ID)
			if zzr(f.options) != "key_key.forward:key.keyboard.w\n" {
				t.Fatal("interrupted apply lost original baseline", zzr(f.options))
			}
		})
	}
}

func TestKeyBindingsMissingAndUnsafeOptions(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "first-run")
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i"})
	if err := applyRootKeyBindings(context.Background(), r, s.globalKeyBindings()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Path, "options.txt")); !os.IsNotExist(err) {
		t.Fatal("created incomplete first-run options")
	}
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), make([]byte, maxGameOptionsSize+1))
	if _, err := s.ReadKeyBindings(context.Background(), r.ID, ""); err == nil {
		t.Fatal("oversized options accepted")
	}
	if err := os.Remove(filepath.Join(r.Path, "options.txt")); err != nil {
		t.Fatal(err)
	}
	t.Run("external-symlink", func(t *testing.T) {
		external := filepath.Join(t.TempDir(), "options.txt")
		writeTestFile(t, external, []byte(bindingFixtureOptions))
		if err := os.Symlink(external, filepath.Join(r.Path, "options.txt")); err != nil {
			t.Skip("symlinks unavailable:", err)
		}
		if err := applyRootKeyBindings(context.Background(), r, s.globalKeyBindings()); err == nil {
			t.Fatal("followed external options symlink")
		}
		if zzr(external) != bindingFixtureOptions {
			t.Fatal("modified external options")
		}
	})
}

func TestKeyBindingsVersionScopesAndNoOp(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "version-scopes")
	ctx := context.Background()
	for _, id := range []string{"one", "two"} {
		writeTestFile(t, filepath.Join(r.Path, "versions", id, id+".json"), []byte(fmt.Sprintf(`{"id":%q,"mainClass":"net.minecraft.client.main.Main"}`, id)))
		writeTestFile(t, filepath.Join(r.Path, "versions", id, "options.txt"), []byte("key_key.forward:key.keyboard.w\nkey_quest.open:key.keyboard.o\n"))
	}
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i"})
	view, err := s.ReadKeyBindings(ctx, r.ID, "one")
	if err != nil {
		t.Fatal(err)
	}
	view, err = s.SaveKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Version: "one", Revision: view.Revision, Bindings: map[string]string{"key_quest.open": "key.keyboard.k"}})
	if err != nil {
		t.Fatal(err)
	}
	view, err = s.ApplyKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Version: "one", Revision: view.Revision})
	if err != nil {
		t.Fatal(err)
	}
	one, two := filepath.Join(r.Path, "versions", "one", "options.txt"), filepath.Join(r.Path, "versions", "two", "options.txt")
	if !strings.Contains(zzr(one), "key_quest.open:key.keyboard.k") || strings.Contains(zzr(two), "key.keyboard.i") {
		t.Fatal("an isolated version's profile modified another version")
	}
	stamp := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(one, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Version: "one", Revision: view.Revision}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(one)
	if err != nil || !info.ModTime().Equal(stamp) {
		t.Fatal("an unchanged profile rewrote options", err)
	}
	if _, err := s.ReadKeyBindings(ctx, r.ID, "removed"); err == nil {
		t.Fatal("missing version fell back to another game's options")
	}
}

func TestKeyBindingsWriteFailureLeavesOptionsIntact(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i"})
	f, err := bindingFiles(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.backup, 0700); err != nil {
		t.Fatal(err)
	}
	if err := applyRootKeyBindings(context.Background(), r, s.globalKeyBindings()); err == nil {
		t.Fatal("continued after backup failure")
	}
	if zzr(f.options) != bindingFixtureOptions {
		t.Fatal("backup failure modified options")
	}
	if err := os.Remove(f.backup); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, f.state, []byte(`{"schema":2,"keys":{}}`))
	if err := applyRootKeyBindings(context.Background(), r, s.globalKeyBindings()); err == nil {
		t.Fatal("unknown state schema accepted")
	}
	if zzr(f.options) != bindingFixtureOptions {
		t.Fatal("corrupt state changed options")
	}
	if err := os.Remove(f.state); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, f.options, make([]byte, maxGameOptionsSize+1))
	if err := applyRootKeyBindings(context.Background(), r, nil); err != nil {
		t.Fatal("inactive feature prevented ordinary launch", err)
	}
}

func TestKeyBindingCatalogAndLegacyRoundTrip(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range keyBindingCatalog.Vanilla {
		for _, id := range append([]string{entry.ID}, entry.Aliases...) {
			if seen[id] || !validBindingID(id) {
				t.Fatal("invalid or duplicate catalog key", id)
			}
			seen[id] = true
		}
		if _, err := NormalizeKeyBinding(entry.Default); err != nil {
			t.Fatal(entry.ID, err)
		}
		for _, en := range []bool{false, true} {
			if label := i18n.Text(en, entry.Label); label == entry.Label || label == "" {
				t.Fatal("missing action translation", entry.Label)
			}
		}
	}
	for code := range keyBindingCatalog.Legacy {
		modern, err := NormalizeKeyBinding(code)
		if err != nil {
			t.Fatal(code, err)
		}
		back, err := bindingForOptions(modern, "17")
		if err != nil || back != code {
			t.Fatal("legacy mapping did not round trip", code, modern, back, err)
		}
	}
}
