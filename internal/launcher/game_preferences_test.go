package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func savePreferences(t *testing.T, s *Service, root string, options map[string]string, apply bool) KeyBindingsView {
	t.Helper()
	ctx := context.Background()
	view, err := s.ReadKeyBindings(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, row := range view.Rows {
		if row.Configured {
			keys[row.ID] = row.Override
		}
	}
	view, err = s.SaveKeyBindings(ctx, KeyBindingsRequest{RootID: root, Revision: view.Revision, Bindings: keys, Options: options, Apply: apply})
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func TestGamePreferencesInheritanceApplyAndRestore(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	path := filepath.Join(r.Path, "options.txt")
	original := bindingFixtureOptions + "\r\nrenderDistance:12\r\nfullscreen:false\r\nfov:0.25\r\n"
	writeTestFile(t, path, []byte(original))
	savePreferences(t, s, "", map[string]string{"renderDistance": "16.000000", "fullscreen": "true", "soundCategory_music": "0.4"}, false)
	view := savePreferences(t, s, r.ID, map[string]string{"renderDistance": "20", "fov": "0.5"}, true)
	got := zzr(path)
	if !strings.Contains(got, "renderDistance:20\r\nfullscreen:true\r\nfov:0.5\r\n") || !strings.Contains(got, "soundCategory_music:0.4") || !strings.Contains(got, "resourcePacks:[\"vanilla\",\"file/a:b.zip\"]") || !strings.Contains(got, "unknownOption:keep:every:colon") {
		t.Fatal("options were not applied selectively", got)
	}
	for _, row := range view.Options {
		if row.ID == "renderDistance" && (row.Override != "20" || row.Inherited != "16" || row.Source != "global") {
			t.Fatal("incorrect global inheritance", row)
		}
	}
	savePreferences(t, s, r.ID, map[string]string{}, true)
	if !strings.Contains(zzr(path), "renderDistance:16\r\n") {
		t.Fatal("integer encoding or inheritance incorrect", zzr(path))
	}
	savePreferences(t, s, "", map[string]string{}, false)
	applyBindings(t, s, r.ID)
	if got := zzr(path); got != original {
		t.Fatal("reset damaged original values or unrelated data", got)
	}
}

func TestGamePreferencesValidationRevisionAndPersistence(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	ctx := context.Background()
	path := filepath.Join(r.Path, "options.txt")
	writeTestFile(t, path, []byte(bindingFixtureOptions+"\nrenderDistance:12\nfullscreen:false\n"))
	view, err := s.ReadKeyBindings(ctx, r.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range []map[string]string{
		{"renderDistance": "NaN"}, {"renderDistance": "33"}, {"renderDistance": "16.5"}, {"fullscreen": "yes"}, {"unknownOption": "bad\nkey_key.forward:x"}, {"resourcePacks": "[]"}, {"newOption": "true"},
	} {
		_, err := s.SaveKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Revision: view.Revision, Options: values, Apply: true})
		if err == nil {
			t.Fatal("accepted invalid settings", values)
		}
	}
	savePreferences(t, s, "", map[string]string{"fullscreen": "true"}, false)
	if _, err = s.SaveKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Revision: view.Revision, Options: map[string]string{}}); err == nil {
		t.Fatal("global option changes did not invalidate an open draft")
	}
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.i"})
	reopened, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.globalManagedOptions()["fullscreen"] != "true" {
		t.Fatal("key-only save erased preferences")
	}
	cloned := reopened.cfg.clone()
	cloned.GamePreferences["fullscreen"] = "false"
	if reopened.globalManagedOptions()["fullscreen"] != "true" {
		t.Fatal("config clone shared preferences")
	}
	savePreferences(t, reopened, r.ID, map[string]string{"unknownOption": "custom:value", "fullscreen": "false"}, true)
	if !strings.Contains(zzr(path), "unknownOption:custom:value") {
		t.Fatal("custom option was not saved")
	}
	s.running[runKey(r.ID, "test")] = &runningGame{}
	v, _ := s.ReadKeyBindings(ctx, r.ID, "")
	if _, err = s.SaveKeyBindings(ctx, KeyBindingsRequest{RootID: r.ID, Revision: v.Revision, Options: map[string]string{"fullscreen": "true"}, Apply: true}); err == nil {
		t.Fatal("edited running game")
	}
	delete(s.running, runKey(r.ID, "test"))
}

func TestGamePreferencesJournalAndLaterGameEdits(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	ctx := context.Background()
	path := filepath.Join(r.Path, "options.txt")
	original := []byte("renderDistance:12\nfullscreen:false\n")
	updated := []byte("renderDistance:18\nfullscreen:true\n")
	writeTestFile(t, path, updated)
	f, err := bindingFiles(r)
	if err != nil {
		t.Fatal(err)
	}
	state := keyBindingState{Schema: 1, Keys: map[string]appliedKeyBinding{}, Pending: &keyBindingPending{
		Before: bindingRevision(nil, nil, original), After: bindingRevision(nil, nil, updated),
		Keys: map[string]appliedKeyBinding{"renderDistance": {Original: "12", Applied: "18"}, "fullscreen": {Original: "false", Applied: "true"}},
	}}
	if err = writeBindingJSON(f.state, state); err != nil {
		t.Fatal(err)
	}
	// The game changed fullscreen after the journal's replacement completed.
	writeTestFile(t, path, []byte("renderDistance:18\nfullscreen:false\nunrelated:keep\n"))
	if err = applyRootKeyBindings(ctx, r, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "renderDistance:12\nfullscreen:false\nunrelated:keep\n" {
		t.Fatal("recovery lost baselines or overwrote newer game edits", string(got))
	}
}
