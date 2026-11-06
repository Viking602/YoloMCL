package launcher

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// GameSettingsBackup describes a saved options file and its override policy.
type GameSettingsBackup struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Reason    string    `json:"reason"`
	Size      int64     `json:"size"`
}

type GameSettingsBackupRequest struct {
	RootID   string `json:"rootId"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
	BackupID string `json:"backupId"`
}

type gameSettingsBackupData struct {
	Options        []byte            `json:"options"`
	OptionsMissing bool              `json:"optionsMissing,omitempty"`
	Schema         int               `json:"schema"`
	Scope          string            `json:"scope"`
	Profile        keyBindingProfile `json:"profile"`
	State          keyBindingState   `json:"state"`
}

const gameSettingsBackupTime = "20060102T150405.000000000Z"
const maxGameSettingsBackupSize = 24 << 20

func globalSettingsBackup(cfg config) gameSettingsBackupData {
	return gameSettingsBackupData{Schema: 1, Scope: "global", Profile: keyBindingProfile{Schema: 1, Bindings: maps.Clone(cfg.KeyBindings), Options: maps.Clone(cfg.GamePreferences)}}
}

func readRootSettingsBackup(ctx context.Context, f keyBindingFiles) (gameSettingsBackupData, error) {
	b := gameSettingsBackupData{Schema: 1, Scope: "instance"}
	var err error
	if b.Options, err = readBindingFile(ctx, f.options); err != nil {
		return b, err
	}
	b.OptionsMissing = b.Options == nil
	if b.Profile, err = readBindingProfile(ctx, f.profile); err != nil {
		return b, err
	}
	if b.State, err = readBindingState(ctx, f.state); err != nil {
		return b, err
	}
	b.State = resolveBindingState(b.State, b.Options)
	return b, nil
}

func rootSettingsBackupDir(f keyBindingFiles) (string, error) {
	return safeJoin(filepath.Dir(f.options), ".mcl/settings-backups")
}

func parseSettingsBackupID(id string) (GameSettingsBackup, bool) {
	parts := strings.Split(id, "-")
	if len(parts) != 3 || !strings.HasSuffix(parts[2], ".json") {
		return GameSettingsBackup{}, false
	}
	created, err := time.Parse(gameSettingsBackupTime, parts[0])
	if err != nil || parts[1] != "manual" && parts[1] != "save" && parts[1] != "restore" {
		return GameSettingsBackup{}, false
	}
	if nonce, err := hex.DecodeString(strings.TrimSuffix(parts[2], ".json")); err != nil || len(nonce) != 16 {
		return GameSettingsBackup{}, false
	}
	return GameSettingsBackup{ID: id, CreatedAt: created, Reason: parts[1]}, true
}

func listSettingsBackups(ctx context.Context, dir string) ([]GameSettingsBackup, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []GameSettingsBackup{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := []GameSettingsBackup{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, valid := parseSettingsBackupID(entry.Name())
		if !valid || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		st, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !st.Mode().IsRegular() {
			continue
		}
		info.Size = st.Size()
		backups = append(backups, info)
	}
	slices.SortFunc(backups, func(a, b GameSettingsBackup) int { return strings.Compare(b.ID, a.ID) })
	return backups, nil
}

func writeSettingsBackup(ctx context.Context, dir, reason string, data gameSettingsBackupData) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(b) > maxGameSettingsBackupSize {
		return keyBindingError("too_many")
	}
	id := time.Now().UTC().Format(gameSettingsBackupTime) + "-" + reason + "-" + newID() + ".json"
	path, err := safeJoin(dir, id)
	if err != nil {
		return err
	}
	if err := atomicBytes(path, b, 0600); err != nil {
		return err
	}
	// Retention is best effort. A cleanup failure must not turn a successfully
	// created safety copy into a failed save. Manual snapshots are never pruned.
	backups, err := listSettingsBackups(ctx, dir)
	if err != nil {
		return nil
	}
	automatic := 0
	for _, backup := range backups {
		if backup.Reason == "manual" {
			continue
		}
		automatic++
		if automatic > 20 {
			if path, err := safeJoin(dir, backup.ID); err == nil {
				_ = os.Remove(path)
			}
		}
	}
	return nil
}

func readSettingsBackup(ctx context.Context, dir, id string) (gameSettingsBackupData, error) {
	var data gameSettingsBackupData
	if _, valid := parseSettingsBackupID(id); !valid {
		return data, keyBindingError("backup_invalid")
	}
	path, err := safeJoin(dir, id)
	if err != nil {
		return data, err
	}
	f, err := os.Open(path)
	if err != nil {
		return data, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return data, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxGameSettingsBackupSize {
		return data, keyBindingError("backup_invalid")
	}
	b, err := io.ReadAll(io.LimitReader(modReader{ctx: ctx, r: f}, maxGameSettingsBackupSize+1))
	if err != nil {
		return data, err
	}
	if len(b) > maxGameSettingsBackupSize {
		return data, keyBindingError("backup_invalid")
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return data, keyBindingError("backup_invalid")
	}
	if data.Schema != 1 || data.Profile.Schema != 1 || data.Scope != "global" && data.Scope != "instance" || len(data.Options) > 1<<20 {
		return data, keyBindingError("backup_invalid")
	}
	if data.Scope == "instance" && ((data.Options == nil) != data.OptionsMissing || data.State.Schema != 1 || data.State.Pending != nil) {
		return data, keyBindingError("backup_invalid")
	}
	global := data.Scope == "global"
	if _, err := validateKeyBindings(data.Profile.Bindings, nil, global); err != nil {
		return data, err
	}
	if _, err := validateGamePreferences(data.Profile.Options, nil, global); err != nil {
		return data, err
	}
	return data, ctx.Err()
}

// GameSettingsBackups lists the selected scope's snapshots without scanning mods.
func (s *Service) GameSettingsBackups(ctx context.Context, rootID, version string) ([]GameSettingsBackup, error) {
	dir := filepath.Join(s.dir, "game-settings-backups")
	if rootID != "" {
		r, err := s.rootAt(rootID, version)
		if err != nil {
			return nil, err
		}
		f, err := bindingFiles(r)
		if err != nil {
			return nil, err
		}
		dir, err = rootSettingsBackupDir(f)
		if err != nil {
			return nil, err
		}
	}
	return listSettingsBackups(ctx, dir)
}
