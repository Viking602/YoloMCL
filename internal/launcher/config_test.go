package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func configFixture(t *testing.T) *Service {
	t.Helper()
	s := testService(t)
	s.cfg.Roots = []Root{
		{ID: "first", Path: t.TempDir(), SelectedVersion: "old", VersionConfigs: map[string]VersionConfig{"old": {DisplayName: "Original"}}},
		{ID: "second", Path: t.TempDir()},
	}
	s.cfg.Accounts = []Account{{ID: "first", Kind: "offline"}, {ID: "second", Kind: "offline"}}
	s.cfg.AuthServers = []AuthServer{{Name: "Original", URL: "https://example.org/auth"}}
	s.cfg.ActiveRoot, s.cfg.ActiveAccount = "first", "first"
	if err := s.saveConfigLocked(s.cfg); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestConfigWritesLeaveStateUnchangedOnFailure(t *testing.T) {
	cases := map[string]func(*testing.T, *Service) error{
		"settings": func(_ *testing.T, s *Service) error {
			next := s.settings()
			next.ShowSnapshots = !next.ShowSnapshots
			return s.SaveSettings(next)
		},
		"selection":            func(_ *testing.T, s *Service) error { return s.Select("second", "second", "new") },
		"root defaults":        func(_ *testing.T, s *Service) error { return s.UpdateRoot("first", "Changed", "", 0) },
		"version overrides":    func(_ *testing.T, s *Service) error { return s.UpdateVersionConfig("first", "old", "java", 0, "") },
		"display name":         func(_ *testing.T, s *Service) error { return s.SetVersionDisplayName("first", "old", "Changed") },
		"background selection": func(_ *testing.T, s *Service) error { return s.setRootVersion("first", "new") },
		"remove root":          func(_ *testing.T, s *Service) error { return s.RemoveRoot("first") },
		"remove account":       func(_ *testing.T, s *Service) error { return s.RemoveAccount("first") },
		"add account":          func(_ *testing.T, s *Service) error { _, err := s.AddOffline("Player"); return err },
		"edit server": func(_ *testing.T, s *Service) error {
			_, err := s.SaveAuthServer("https://example.org/auth", "Changed", "https://example.org/auth")
			return err
		},
		"remove server": func(_ *testing.T, s *Service) error { return s.DeleteAuthServer("https://example.org/auth") },
		"add root": func(t *testing.T, s *Service) error {
			dir := filepath.Join(t.TempDir(), ".minecraft")
			if err := os.MkdirAll(filepath.Join(dir, "versions"), 0700); err != nil {
				t.Fatal(err)
			}
			_, err := s.AddRoot(dir, "New")
			return err
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			s := configFixture(t)
			before := s.cfg.clone()
			path := filepath.Join(s.dir, "config.json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil { // atomic replacement must fail on every OS
				t.Fatal(err)
			}
			if err := change(t, s); err == nil {
				t.Fatal("configuration write unexpectedly succeeded")
			}
			if !reflect.DeepEqual(s.cfg, before) {
				t.Fatal("failed write modified the live configuration")
			}
		})
	}
}

func TestSelectValidatesWholeChangeBeforePublishing(t *testing.T) {
	s := configFixture(t)
	before := s.cfg.clone()
	if err := s.Select("second", "missing-account", "new"); err == nil {
		t.Fatal("invalid account accepted")
	}
	if !reflect.DeepEqual(before, s.cfg) {
		t.Fatal("invalid account partially changed root selection")
	}
	if err := s.Select("second", "second", "new"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, s.cfg) {
		t.Fatal("memory and persisted configuration differ")
	}
	if err := s.setRootVersion("first", "installed"); err != nil {
		t.Fatal(err)
	}
	if s.cfg.ActiveRoot != "second" {
		t.Fatal("background install changed the active instance")
	}
}

func TestConfigReadersDoNotExposeVersionMaps(t *testing.T) {
	s := configFixture(t)
	r, err := s.getRoot("first")
	if err != nil {
		t.Fatal(err)
	}
	r.VersionConfigs["old"] = VersionConfig{DisplayName: "Changed by caller"}
	snap := s.Snapshot()
	snap.Instances[0].VersionConfigs["old"] = VersionConfig{DisplayName: "Changed by view"}
	if s.cfg.Roots[0].VersionConfigs["old"].DisplayName != "Original" {
		t.Fatal("reader mutated the shared configuration")
	}
}
