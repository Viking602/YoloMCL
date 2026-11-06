package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type KeyBindingRow struct {
	Names      map[string]string `json:"names,omitempty"`
	ModName    string            `json:"modName,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	ID         string            `json:"id"`
	Label      string            `json:"label,omitempty"`
	Current    string            `json:"current"`
	Inherited  string            `json:"inherited"`
	Override   string            `json:"override,omitempty"`
	Configured bool              `json:"configured"`
	Vanilla    bool              `json:"vanilla"`
	Source     string            `json:"source"`
}

type KeyBindingsView struct {
	Options          []GamePreferenceRow `json:"options"`
	LabelsIncomplete bool                `json:"labelsIncomplete,omitempty"`
	options          []byte
	minecraft        string
	Revision         string          `json:"revision"`
	Rows             []KeyBindingRow `json:"rows"`
	HasOptions       bool            `json:"hasOptions"`
	Modifiers        bool            `json:"modifiers"`
	Shared           bool            `json:"shared"`
}

type KeyBindingsRequest struct {
	Options  map[string]string `json:"options"`
	Apply    bool              `json:"apply"`
	RootID   string            `json:"rootId"`
	Version  string            `json:"version"`
	Revision string            `json:"revision"`
	Bindings map[string]string `json:"bindings"`
}

type keyBindingProfile struct {
	Options  map[string]string `json:"options,omitempty"`
	Schema   int               `json:"schema"`
	Bindings map[string]string `json:"bindings"`
}

type appliedKeyBinding struct {
	Original string `json:"original"`
	Applied  string `json:"applied"`
}

type keyBindingState struct {
	Schema  int                          `json:"schema"`
	Keys    map[string]appliedKeyBinding `json:"keys"`
	Pending *keyBindingPending           `json:"pending,omitempty"`
}

type keyBindingPending struct {
	Before string                       `json:"before"`
	After  string                       `json:"after"`
	Keys   map[string]appliedKeyBinding `json:"keys"`
}

func resolveBindingState(state keyBindingState, data []byte) keyBindingState {
	if pending := state.Pending; pending != nil {
		if bindingRevision(nil, nil, data) == pending.After {
			state.Keys = pending.Keys
		} else if bindingRevision(nil, nil, data) != pending.Before {
			// A game or editor may have touched unrelated options after the
			// replacement. Recover each binding whose planned value survived.
			keys := parseManagedOptions(data)
			state.Keys = maps.Clone(state.Keys)
			if state.Keys == nil {
				state.Keys = map[string]appliedKeyBinding{}
			}
			for key, value := range pending.Keys {
				if keys[key] == value.Applied {
					state.Keys[key] = value
				}
			}
		}
		state.Pending = nil
	}
	return state
}

type keyBindingFiles struct{ options, profile, state, backup string }

func bindingFiles(r Root) (keyBindingFiles, error) {
	dir, err := filepath.Rel(r.Path, r.VersionDir())
	if err != nil {
		return keyBindingFiles{}, err
	}
	var f keyBindingFiles
	for name, out := range map[string]*string{"options.txt": &f.options, ".mcl/keybindings.json": &f.profile, ".mcl/keybindings-state.json": &f.state, ".mcl/keybindings-options.bak": &f.backup} {
		*out, err = safeJoin(r.Path, filepath.ToSlash(filepath.Join(dir, name)))
		if err != nil {
			return keyBindingFiles{}, err
		}
	}
	return f, nil
}

func readBindingFile(ctx context.Context, path string) ([]byte, error) {
	return readOptionalGameFile(ctx, filepath.Dir(path), filepath.Base(path))
}

// The journal contains both the previous and next baselines, plus JSON
// escaping. It needs more space than the 1 MiB options.txt input limit.
const maxBindingJSONSize = 8 << 20

func readBindingJSON(ctx context.Context, path string, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxBindingJSONSize+1))
	if err != nil {
		return err
	}
	if len(b) > maxBindingJSONSize {
		return keyBindingError("too_many")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}

func writeBindingJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > maxBindingJSONSize {
		return keyBindingError("too_many")
	}
	return atomicBytes(path, b, 0600)
}

func readBindingProfile(ctx context.Context, path string) (keyBindingProfile, error) {
	p := keyBindingProfile{Schema: 1, Bindings: map[string]string{}}
	err := readBindingJSON(ctx, path, &p)
	if err == nil && p.Schema != 1 {
		err = keyBindingError("schema")
	}
	return p, err
}

func readBindingState(ctx context.Context, path string) (keyBindingState, error) {
	p := keyBindingState{Schema: 1, Keys: map[string]appliedKeyBinding{}}
	err := readBindingJSON(ctx, path, &p)
	if err == nil && p.Schema != 1 {
		err = keyBindingError("schema")
	}
	return p, err
}

func parseKeyBindings(data []byte) map[string]string {
	keys := map[string]string{}
	for _, line := range strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r"), ":")
		if ok && validBindingID(key) {
			keys[key] = value
		}
	}
	return keys
}

func bindingRevision(global, local map[string]string, data []byte) string {
	b, _ := json.Marshal([]map[string]string{global, local})
	h := sha256.New()
	h.Write(b)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func bindingDisplay(value string) string {
	if n, err := NormalizeKeyBinding(value); err == nil {
		return n
	}
	return value
}

func (s *Service) globalKeyBindings() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.cfg.KeyBindings)
}

func (s *Service) ReadKeyBindings(ctx context.Context, rootID, version string) (KeyBindingsView, error) {
	out, err := s.ReadKeyBindingValues(ctx, rootID, version)
	if err != nil || rootID == "" {
		return out, err
	}
	r, err := s.rootAt(rootID, version)
	if err == nil {
		err = s.resolveKeyBindingLabels(ctx, r, &out)
	}
	return out, err
}

// ReadKeyBindingValues reads editable settings without opening mod archives.
// Names can be loaded independently through ReadKeyBindings.
func (s *Service) ReadKeyBindingValues(ctx context.Context, rootID, version string) (KeyBindingsView, error) {
	global := s.globalManagedOptions()
	out := KeyBindingsView{Rows: []KeyBindingRow{}}
	if rootID == "" {
		out.Revision = bindingRevision(global, nil, nil)
		for _, b := range keyBindingCatalog.Vanilla {
			value, configured := global[b.ID]
			out.Rows = append(out.Rows, KeyBindingRow{ID: b.ID, Label: b.Label, Vanilla: true, Override: value, Configured: configured, Inherited: b.Default, Source: "game"})
		}
		out.Options = globalPreferenceRows(global)
		return out, ctx.Err()
	}
	r, err := s.rootAt(rootID, version)
	if err != nil {
		return out, err
	}
	return readRootKeyBindings(ctx, r, global)
}

func readRootKeyBindings(ctx context.Context, r Root, global map[string]string) (KeyBindingsView, error) {
	out := KeyBindingsView{Rows: []KeyBindingRow{}}
	versions := scanVersions(r.Path)
	if r.SelectedVersion != "" && !slices.ContainsFunc(versions, func(v VersionSummary) bool { return v.ID == r.SelectedVersion }) {
		return out, keyBindingError("version_missing")
	}
	f, err := bindingFiles(r)
	if err != nil {
		return out, err
	}
	b, err := readBindingFile(ctx, f.options)
	if err != nil {
		return out, err
	}
	p, err := readBindingProfile(ctx, f.profile)
	if err != nil {
		return out, err
	}
	state, err := readBindingState(ctx, f.state)
	if err != nil {
		return out, err
	}
	state = resolveBindingState(state, b)
	out.HasOptions = b != nil
	out.options = b
	out.Shared = filepath.Clean(r.Path) == filepath.Clean(r.VersionDir()) && len(versions) > 1
	out.Revision = bindingRevision(global, managedValues(p.Bindings, p.Options), b)
	out.Options = preferenceRows(b, global, p.Options, state)
	keys := parseKeyBindings(b)
	for key, current := range keys {
		row := KeyBindingRow{ID: key, Current: bindingDisplay(current), Inherited: bindingDisplay(current), Source: "game"}
		if last, ok := state.Keys[key]; ok && last.Applied == current {
			row.Inherited = bindingDisplay(last.Original)
		}
		if base, ok := vanillaBinding(key); ok {
			row.Vanilla, row.Label = true, base.Label
			if v, configured := global[base.ID]; configured {
				row.Inherited, row.Source = v, "global"
			}
		}
		row.Override, row.Configured = p.Bindings[key]
		out.Modifiers = out.Modifiers || strings.Contains(current, ":")
		out.Rows = append(out.Rows, row)
	}
	for _, v := range versions {
		if v.ID == r.SelectedVersion {
			out.minecraft = v.Minecraft
			out.Modifiers = out.Modifiers || v.Loader == "forge" || v.Loader == "neoforge"
		}
	}
	slices.SortFunc(out.Rows, func(a, b KeyBindingRow) int {
		if a.Vanilla != b.Vanilla {
			if a.Vanilla {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func validateKeyBindings(bindings map[string]string, allowed map[string]bool, global bool) (map[string]string, error) {
	if len(bindings) > 4096 {
		return nil, keyBindingError("too_many")
	}
	next := map[string]string{}
	for key, value := range bindings {
		if !validBindingID(key) || allowed != nil && !allowed[key] {
			return nil, keyBindingError("unknown_key", key)
		}
		if global {
			b, ok := vanillaBinding(key)
			if !ok || b.ID != key || strings.Contains(value, ":") {
				return nil, keyBindingError("vanilla_only")
			}
		}
		normal, err := NormalizeKeyBinding(value)
		if err != nil {
			return nil, err
		}
		next[key] = normal
	}
	return next, nil
}

// SaveKeyBindings persists only selected overrides. An absent entry follows
// global/game values; key.keyboard.unknown explicitly unbinds the action.
func (s *Service) SaveKeyBindings(ctx context.Context, req KeyBindingsRequest) (KeyBindingsView, error) {
	if req.RootID == "" {
		next, err := validateKeyBindings(req.Bindings, nil, true)
		if err != nil {
			return KeyBindingsView{}, err
		}
		s.mu.Lock()
		if req.Revision == "" || req.Revision != bindingRevision(managedValues(s.cfg.KeyBindings, s.cfg.GamePreferences), nil, nil) {
			s.mu.Unlock()
			return KeyBindingsView{}, keyBindingError("stale")
		}
		draft := s.cfg.clone()
		draft.KeyBindings = next
		if req.Options != nil {
			draft.GamePreferences, err = validateGamePreferences(req.Options, nil, true)
			if err != nil {
				s.mu.Unlock()
				return KeyBindingsView{}, err
			}
		}
		err = ctx.Err()
		if err == nil && (!maps.Equal(draft.KeyBindings, s.cfg.KeyBindings) || !maps.Equal(draft.GamePreferences, s.cfg.GamePreferences)) {
			err = writeSettingsBackup(ctx, filepath.Join(s.dir, "game-settings-backups"), "save", globalSettingsBackup(s.cfg))
		}
		if err == nil {
			err = s.saveConfigLocked(draft)
		}
		s.mu.Unlock()
		if err != nil {
			return KeyBindingsView{}, err
		}
	} else {
		err := s.mutateRoot(req.RootID, func(r Root) error {
			if req.Version != "" {
				r.SelectedVersion = req.Version
				s.cacheGameDir(&r)
			}
			global := s.globalManagedOptions()
			view, err := readRootKeyBindings(ctx, r, global)
			if err != nil {
				return err
			}
			if req.Revision == "" || req.Revision != view.Revision {
				return keyBindingError("stale")
			}
			allowed := map[string]bool{}
			for _, row := range view.Rows {
				allowed[row.ID] = true
			}
			next, err := validateKeyBindings(req.Bindings, allowed, false)
			if err != nil {
				return err
			}
			f, err := bindingFiles(r)
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			profile, err := readBindingProfile(ctx, f.profile)
			if err != nil {
				return err
			}
			profile.Bindings = next
			if req.Options != nil {
				allowedOptions := map[string]bool{}
				for _, row := range view.Options {
					allowedOptions[row.ID] = true
				}
				profile.Options, err = validateGamePreferences(req.Options, allowedOptions, false)
				if err != nil {
					return err
				}
			}
			if err := backupRootGameSettings(ctx, f, "save"); err != nil {
				return err
			}
			if err := writeBindingJSON(f.profile, profile); err != nil {
				return err
			}
			if req.Apply {
				return applyRootKeyBindings(ctx, r, global)
			}
			return nil
		})
		if err != nil {
			return KeyBindingsView{}, err
		}
	}
	return s.ReadKeyBindingValues(ctx, req.RootID, req.Version)
}

// ApplyKeyBindings rechecks the displayed revision under the root lock.
func (s *Service) ApplyKeyBindings(ctx context.Context, req KeyBindingsRequest) (KeyBindingsView, error) {
	err := s.mutateRoot(req.RootID, func(r Root) error {
		if req.Version != "" {
			r.SelectedVersion = req.Version
			s.cacheGameDir(&r)
		}
		global := s.globalManagedOptions()
		view, err := readRootKeyBindings(ctx, r, global)
		if err != nil {
			return err
		}
		if req.Revision == "" || req.Revision != view.Revision {
			return keyBindingError("stale")
		}
		return applyRootKeyBindings(ctx, r, global)
	})
	if err != nil {
		return KeyBindingsView{}, err
	}
	return s.ReadKeyBindingValues(ctx, req.RootID, req.Version)
}

func applyRootKeyBindings(ctx context.Context, r Root, global map[string]string) error {
	if len(global) == 0 {
		_, profileErr := os.Lstat(filepath.Join(r.VersionDir(), ".mcl", "keybindings.json"))
		_, stateErr := os.Lstat(filepath.Join(r.VersionDir(), ".mcl", "keybindings-state.json"))
		if os.IsNotExist(profileErr) && os.IsNotExist(stateErr) {
			return ctx.Err()
		}
	}
	f, err := bindingFiles(r)
	if err != nil {
		return err
	}
	b, err := readBindingFile(ctx, f.options)
	// Let Minecraft create its version-specific options on its first run.
	if err != nil || b == nil {
		return err
	}
	p, err := readBindingProfile(ctx, f.profile)
	if err != nil {
		return err
	}
	state, err := readBindingState(ctx, f.state)
	if err != nil {
		return err
	}
	hadPending := state.Pending != nil
	state = resolveBindingState(state, b)
	global, err = validateManagedValues(global, true)
	if err != nil {
		return err
	}
	p.Bindings, err = validateKeyBindings(p.Bindings, nil, false)
	if err != nil {
		return err
	}
	p.Options, err = validateGamePreferences(p.Options, nil, false)
	if err != nil {
		return err
	}
	local := managedValues(p.Bindings, p.Options)
	keys := parseManagedOptions(b)
	next := keyBindingState{Schema: 1, Keys: map[string]appliedKeyBinding{}}
	replacements := map[string]string{}
	for key, current := range keys {
		value, managed := local[key]
		if !managed {
			if base, ok := vanillaBinding(key); ok {
				value, managed = global[base.ID]
			} else if !validBindingID(key) {
				value, managed = global[key]
			}
		}
		last, applied := state.Keys[key]
		if !managed {
			if applied && current == last.Applied {
				replacements[key] = last.Original
			}
			continue
		}
		if validBindingID(key) {
			value, err = bindingForOptions(value, current)
			if err != nil {
				return err
			}
		}
		original := current
		if applied && current == last.Applied {
			original = last.Original
		}
		next.Keys[key] = appliedKeyBinding{Original: original, Applied: value}
		replacements[key] = value
	}
	updated := patchKeyBindings(b, replacements)
	if !bytes.Equal(b, updated) {
		if err = atomicBytes(f.backup, b, 0600); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	check, err := readBindingFile(ctx, f.options)
	if err != nil {
		return err
	}
	if !bytes.Equal(b, check) {
		return keyBindingError("stale")
	}
	if bytes.Equal(b, updated) {
		if hadPending || !maps.Equal(state.Keys, next.Keys) {
			return writeBindingJSON(f.state, next)
		}
		return nil
	}
	// The journal retains both baselines until the atomic options replacement
	// is committed. Recovery works whether the process stops before or after it.
	journal := state
	journal.Pending = &keyBindingPending{Before: bindingRevision(nil, nil, b), After: bindingRevision(nil, nil, updated), Keys: next.Keys}
	if err = writeBindingJSON(f.state, journal); err != nil {
		return err
	}
	mode := os.FileMode(0644)
	if info, e := os.Stat(f.options); e == nil {
		mode = info.Mode().Perm()
	}
	if err = atomicBytes(f.options, updated, mode); err != nil {
		return err
	}
	return writeBindingJSON(f.state, next)
}

func patchKeyBindings(data []byte, values map[string]string) []byte {
	lines := strings.SplitAfter(string(data), "\n")
	for i, line := range lines {
		prefix := ""
		if i == 0 && strings.HasPrefix(line, "\ufeff") {
			prefix, line = "\ufeff", strings.TrimPrefix(line, "\ufeff")
		}
		body := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		key, _, ok := strings.Cut(body, ":")
		if value, change := values[key]; ok && change {
			lines[i] = prefix + key + ":" + value + line[len(body):]
		}
	}
	return []byte(strings.Join(lines, ""))
}
