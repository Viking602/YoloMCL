package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func backupRequest(t *testing.T, s *Service, root, id string) GameSettingsBackupRequest {
	t.Helper()
	v, err := s.ReadKeyBindingValues(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	return GameSettingsBackupRequest{RootID: root, Revision: v.Revision, BackupID: id}
}

func TestGameSettingsBackupRestoreFileAndOverrides(t *testing.T) {
	s, ctx := testService(t), context.Background()
	r := bindingFixture(t, s)
	path := filepath.Join(r.Path, "options.txt")
	original := bindingFixtureOptions + "\r\nmaxFps:120\r\nmouseSensitivity:0.204225\r\n"
	writeTestFile(t, path, []byte(original))
	backups, err := s.BackupGameSettings(ctx, backupRequest(t, s, r.ID, ""))
	if err != nil || len(backups) != 1 || backups[0].Reason != "manual" {
		t.Fatal(backups, err)
	}
	originalID := backups[0].ID
	v := savePreferences(t, s, r.ID, map[string]string{"maxFps": "260", "mouseSensitivity": "0.205"}, true)
	changed := zzr(path)
	if !strings.Contains(changed, "maxFps:260") {
		t.Fatal(changed)
	}
	backups, err = s.GameSettingsBackups(ctx, r.ID, "")
	if err != nil || len(backups) != 2 || backups[0].Reason != "save" {
		t.Fatal(backups, err)
	}
	v, err = s.RestoreGameSettings(ctx, GameSettingsBackupRequest{RootID: r.ID, Revision: v.Revision, BackupID: originalID})
	if err != nil || zzr(path) != original {
		t.Fatal("restore lost original bytes", err, zzr(path))
	}
	for _, row := range v.Options {
		if row.Configured {
			t.Fatal("old override survived restore", row)
		}
	}
	applyBindings(t, s, r.ID)
	if zzr(path) != original {
		t.Fatal("launch reapplied discarded overrides")
	}
	backups, err = s.GameSettingsBackups(ctx, r.ID, "")
	if err != nil || backups[0].Reason != "restore" {
		t.Fatal(backups, err)
	}
	if _, err = s.RestoreGameSettings(ctx, backupRequest(t, s, r.ID, backups[0].ID)); err != nil {
		t.Fatal(err)
	}
	applyBindings(t, s, r.ID)
	if zzr(path) != changed {
		t.Fatal("could not undo restore", zzr(path))
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreGameSettings(ctx, backupRequest(t, s, r.ID, originalID)); err != nil || zzr(path) != original {
		t.Fatal("could not restore missing options.txt", err)
	}
}

func TestGameSettingsGlobalBackupKeepsUnrelatedConfiguration(t *testing.T) {
	s, ctx := testService(t), context.Background()
	saveBindings(t, s, "", map[string]string{"key_key.forward": "key.keyboard.g"})
	savePreferences(t, s, "", map[string]string{"maxFps": "260"}, false)
	backups, err := s.BackupGameSettings(ctx, backupRequest(t, s, "", ""))
	if err != nil {
		t.Fatal(err)
	}
	id := backups[0].ID
	data := zzr(filepath.Join(s.dir, "game-settings-backups", id))
	if strings.Contains(data, `"accounts"`) || strings.Contains(data, `"roots"`) || strings.Contains(data, `"settings"`) || !strings.Contains(data, `"maxFps":"260"`) {
		t.Fatal("backup contains unrelated config or lost settings", data)
	}
	saveBindings(t, s, "", nil)
	savePreferences(t, s, "", map[string]string{"maxFps": "60"}, false)
	s.mu.Lock()
	s.cfg.Settings.JVMArgs = "-Dbackup.regression=true"
	s.mu.Unlock()
	if _, err = s.RestoreGameSettings(ctx, backupRequest(t, s, "", id)); err != nil {
		t.Fatal(err)
	}
	if got := s.globalManagedOptions(); got["maxFps"] != "260" || got["key_key.forward"] != "key.keyboard.g" {
		t.Fatal(got)
	}
	if s.settings().JVMArgs != "-Dbackup.regression=true" {
		t.Fatal("restore changed launcher settings")
	}
	reopened, err := New(s.dir)
	if err != nil || reopened.globalManagedOptions()["maxFps"] != "260" {
		t.Fatal("restore did not persist", err)
	}
}

func TestGameSettingsBackupRestoresEmptyOptionsFile(t *testing.T) {
	s, ctx := testService(t), context.Background()
	r := bindingFixture(t, s)
	path := filepath.Join(r.Path, "options.txt")
	writeTestFile(t, path, []byte{})
	items, err := s.BackupGameSettings(ctx, backupRequest(t, s, r.ID, ""))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, []byte("maxFps:120\n"))
	if _, err := s.RestoreGameSettings(ctx, backupRequest(t, s, r.ID, items[0].ID)); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil || len(b) != 0 {
		t.Fatal("empty options file was removed or not restored", err)
	}
}

func TestGameSettingsBackupRejectsStaleCorruptAndCrossPackRestores(t *testing.T) {
	s, ctx := testService(t), context.Background()
	r := bindingFixture(t, s)
	backups, err := s.BackupGameSettings(ctx, backupRequest(t, s, r.ID, ""))
	if err != nil {
		t.Fatal(err)
	}
	id := backups[0].ID
	stale := backupRequest(t, s, r.ID, id)
	path := filepath.Join(r.Path, "options.txt")
	external := bindingFixtureOptions + "\nmaxFps:60\n"
	writeTestFile(t, path, []byte(external))
	if _, err = s.RestoreGameSettings(ctx, stale); err == nil {
		t.Fatal("stale restore overwrote external edit")
	}
	for _, bad := range []string{"../options.txt", "..\\config.json", "", id + "/options.txt"} {
		if _, err = s.RestoreGameSettings(ctx, backupRequest(t, s, r.ID, bad)); err == nil {
			t.Fatal("accepted unsafe ID", bad)
		}
	}
	other := bindingFixture(t, s)
	if _, err = s.RestoreGameSettings(ctx, backupRequest(t, s, other.ID, id)); err == nil {
		t.Fatal("cross-pack restore accepted")
	}
	s.mu.Lock()
	s.rootLocks[r.ID] = true
	s.mu.Unlock()
	_, err = s.RestoreGameSettings(ctx, backupRequest(t, s, r.ID, id))
	s.mu.Lock()
	delete(s.rootLocks, r.ID)
	s.mu.Unlock()
	if err == nil {
		t.Fatal("restored while pack busy")
	}
	writeTestFile(t, filepath.Join(r.Path, ".mcl", "settings-backups", id), []byte(`{"schema":999}`))
	if _, err = s.RestoreGameSettings(ctx, backupRequest(t, s, r.ID, id)); err == nil {
		t.Fatal("corrupt backup accepted")
	}
	if zzr(path) != external {
		t.Fatal("failed restore changed current options")
	}
	backups, err = s.GameSettingsBackups(ctx, r.ID, "")
	if err != nil || len(backups) != 1 {
		t.Fatal("failed restore created backups", backups, err)
	}
}

func TestGameSettingsAutomaticRetentionPreservesManualBackups(t *testing.T) {
	dir := t.TempDir()
	data := gameSettingsBackupData{Schema: 1, Scope: "global", Profile: keyBindingProfile{Schema: 1}}
	if err := writeSettingsBackup(context.Background(), dir, "manual", data); err != nil {
		t.Fatal(err)
	}
	for range 23 {
		if err := writeSettingsBackup(context.Background(), dir, "save", data); err != nil {
			t.Fatal(err)
		}
	}
	items, err := listSettingsBackups(context.Background(), dir)
	if err != nil || len(items) != 21 || items[len(items)-1].Reason != "manual" {
		t.Fatal(items, err)
	}
}

func TestGameSettingsBackupFailureDoesNotChangeOptions(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	writeTestFile(t, filepath.Join(r.Path, ".mcl", "settings-backups"), []byte("occupied"))
	v, err := s.ReadKeyBindingValues(context.Background(), r.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveKeyBindings(context.Background(), KeyBindingsRequest{RootID: r.ID, Revision: v.Revision, Bindings: map[string]string{"key_quest.open": "key.keyboard.q"}, Apply: true})
	if err == nil || zzr(filepath.Join(r.Path, "options.txt")) != bindingFixtureOptions {
		t.Fatal("unsafe save without backup", err)
	}
	if _, err = os.Stat(filepath.Join(r.Path, ".mcl", "keybindings.json")); !os.IsNotExist(err) {
		t.Fatal("failed save changed override profile", err)
	}
}

func TestGameSettingsReadAndSaveDoNotWaitForLabels(t *testing.T) {
	s := testService(t)
	r := bindingFixture(t, s)
	s.keyBindingLabels.Lock()
	defer s.keyBindingLabels.Unlock()
	done := make(chan error, 1)
	go func() {
		v, err := s.ReadKeyBindingValues(context.Background(), r.ID, "")
		if err == nil {
			_, err = s.SaveKeyBindings(context.Background(), KeyBindingsRequest{RootID: r.ID, Revision: v.Revision, Bindings: map[string]string{"key_quest.open": "key.keyboard.q"}, Apply: true})
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("settings read/save waited for translation cache")
	}
}
