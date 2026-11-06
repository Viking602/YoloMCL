package launcher

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
)

// BackupGameSettings creates a manual snapshot of saved settings, not UI drafts.
func (s *Service) BackupGameSettings(ctx context.Context, req GameSettingsBackupRequest) ([]GameSettingsBackup, error) {
	if err := s.changeGameSettingsBackup(ctx, req, false); err != nil {
		return nil, err
	}
	return s.GameSettingsBackups(ctx, req.RootID, req.Version)
}

// RestoreGameSettings restores a snapshot after first backing up the current state.
func (s *Service) RestoreGameSettings(ctx context.Context, req GameSettingsBackupRequest) (KeyBindingsView, error) {
	if err := s.changeGameSettingsBackup(ctx, req, true); err != nil {
		return KeyBindingsView{}, err
	}
	return s.ReadKeyBindingValues(ctx, req.RootID, req.Version)
}

func (s *Service) changeGameSettingsBackup(ctx context.Context, req GameSettingsBackupRequest, restore bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.RootID == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		if req.Revision == "" || req.Revision != bindingRevision(managedValues(s.cfg.KeyBindings, s.cfg.GamePreferences), nil, nil) {
			return keyBindingError("stale")
		}
		dir := filepath.Join(s.dir, "game-settings-backups")
		if !restore {
			return writeSettingsBackup(ctx, dir, "manual", globalSettingsBackup(s.cfg))
		}
		backup, err := readSettingsBackup(ctx, dir, req.BackupID)
		if err != nil {
			return err
		}
		if backup.Scope != "global" {
			return keyBindingError("backup_invalid")
		}
		if err := writeSettingsBackup(ctx, dir, "restore", globalSettingsBackup(s.cfg)); err != nil {
			return err
		}
		draft := s.cfg.clone()
		draft.KeyBindings, draft.GamePreferences = backup.Profile.Bindings, backup.Profile.Options
		if err := ctx.Err(); err != nil {
			return err
		}
		return s.saveConfigLocked(draft)
	}
	return s.mutateRoot(req.RootID, func(r Root) error {
		if req.Version != "" {
			r.SelectedVersion = req.Version
			s.cacheGameDir(&r)
		}
		view, err := readRootKeyBindings(ctx, r, s.globalManagedOptions())
		if err != nil {
			return err
		}
		if req.Revision == "" || req.Revision != view.Revision {
			return keyBindingError("stale")
		}
		if !view.HasOptions && !restore {
			return keyBindingError("backup_no_options")
		}
		f, err := bindingFiles(r)
		if err != nil {
			return err
		}
		dir, err := rootSettingsBackupDir(f)
		if err != nil {
			return err
		}
		current, err := readRootSettingsBackup(ctx, f)
		if err != nil {
			return err
		}
		if !bytes.Equal(current.Options, view.options) {
			return keyBindingError("stale")
		}
		if !restore {
			return writeSettingsBackup(ctx, dir, "manual", current)
		}
		backup, err := readSettingsBackup(ctx, dir, req.BackupID)
		if err != nil {
			return err
		}
		if backup.Scope != "instance" {
			return keyBindingError("backup_invalid")
		}
		if err := writeSettingsBackup(ctx, dir, "restore", current); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		check, err := readBindingFile(ctx, f.options)
		if err != nil {
			return err
		}
		if !bytes.Equal(check, current.Options) {
			return keyBindingError("stale")
		}
		// Finish or roll back the three files together. Do not let cancellation
		// interrupt the commit after the first replacement. The safety copy
		// remains available even if an IO failure also prevents rollback.
		if err := writeRestoredSettings(f, backup); err != nil {
			return errors.Join(err, writeRestoredSettings(f, current))
		}
		return nil
	})
}

func writeRestoredSettings(f keyBindingFiles, data gameSettingsBackupData) error {
	if err := writeBindingJSON(f.profile, data.Profile); err != nil {
		return err
	}
	if err := writeBindingJSON(f.state, data.State); err != nil {
		return err
	}
	if data.OptionsMissing {
		err := os.Remove(f.options)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	mode := os.FileMode(0644)
	if st, err := os.Stat(f.options); err == nil {
		mode = st.Mode().Perm()
	}
	return atomicBytes(f.options, data.Options, mode)
}

func backupRootGameSettings(ctx context.Context, f keyBindingFiles, reason string) error {
	data, err := readRootSettingsBackup(ctx, f)
	if err != nil {
		return err
	}
	if data.Options == nil {
		return nil
	}
	dir, err := rootSettingsBackupDir(f)
	if err != nil {
		return err
	}
	return writeSettingsBackup(ctx, dir, reason, data)
}
