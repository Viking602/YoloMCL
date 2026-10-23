package nativeui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func systemLanguageTester(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	t.Setenv("MCL_LANG", "")
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	if a.settings.Language != "" {
		t.Fatal("new settings must follow the system")
	}
	a.systemLanguages = func() []string { return []string{"zh-Hans-CN", "en-US"} }
	a.refreshSystemLanguage()
	a.page = "settings"
	return a, ui.NewTester(a.View, 1320, 860)
}

func TestSystemLanguageManualSelectionAndResume(t *testing.T) {
	a, tt := systemLanguageTester(t)
	if a.lang() != LangZH || !tt.HasText("跟随系统") {
		t.Fatal("Chinese system preference was not applied")
	}
	clickUI(t, tt, "English")
	a.bgWG.Wait()
	tt.Frame()
	if a.lang() != LangEN || a.settings.Language != LangEN {
		t.Fatal("manual English selection was not applied")
	}
	a.systemLanguages = func() []string { return []string{"zh-Hant-TW"} }
	tt.Frame()
	if a.lang() != LangEN {
		t.Fatal("system change overwrote manual choice")
	}
	clickUI(t, tt, "System")
	a.bgWG.Wait()
	tt.Frame()
	if a.lang() != LangZH || a.settings.Language != "" {
		t.Fatal("following the system did not resume")
	}
	a.systemLanguages = func() []string { return []string{"en-GB", "zh-CN"} }
	tt.Frame()
	if a.lang() != LangEN || !tt.HasText("Runtime Environment") || a.resourceType != enCopy.DiscoverTypeModpack {
		t.Fatal("system language change did not update UI and filter labels")
	}
	// Mode stays automatic even when the effective language is English.
	if a.service.Snapshot().Settings.Language != "" {
		t.Fatal("system language was persisted as a manual choice")
	}
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "language-system-en")
	clickUI(t, tt, "简体中文")
	a.bgWG.Wait()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "language-manual-zh")
}

func TestSystemLanguageSurvivesOtherSettingsAndRestart(t *testing.T) {
	a, tt := systemLanguageTester(t)
	a.memoryText = "2048"
	a.settings.ShowSnapshots = true
	a.persistSettings("")
	a.bgWG.Wait()
	tt.Frame()
	s, err := launcher.New(a.snapshot.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Settings; got.Language != "" || got.MemoryMB != 2048 || !got.ShowSnapshots {
		t.Fatalf("other settings froze automatic language or failed to persist: %+v", got)
	}
	reopened, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.cancel()
	reopened.systemLanguages = func() []string { return []string{"en-US"} }
	reopened.refreshSystemLanguage()
	if reopened.lang() != LangEN {
		t.Fatal("restart did not re-resolve system language")
	}
	a.setLanguage(LangEN)
	a.bgWG.Wait()
	tt.Frame()
	clickUI(t, tt, a.tr().SettingsResetBtn)
	a.bgWG.Wait()
	tt.Frame()
	if a.settings.Language != "" || a.lang() != LangZH {
		t.Fatal("reset did not restore system mode")
	}
}

func TestSystemLanguageSharesSettingsWriteOrder(t *testing.T) {
	a, tt := systemLanguageTester(t)
	a.settingsMu.Lock()
	a.setLanguage(LangEN)
	a.memoryText = "2048"
	a.persistSettings("")
	a.setLanguage("")
	a.settingsMu.Unlock()
	a.bgWG.Wait()
	tt.Frame()
	if got := a.service.Snapshot().Settings; got.Language != "" || got.MemoryMB != 2048 {
		t.Fatalf("an older language/settings draft won: %+v", got)
	}
}

func TestLanguageOverrideDoesNotPersistWithOtherSettings(t *testing.T) {
	a, tt := systemLanguageTester(t)
	a.OverrideLanguage(LangEN)
	a.memoryText = "2048"
	a.persistSettings("")
	a.bgWG.Wait()
	tt.Frame()
	if a.lang() != LangEN || a.service.Snapshot().Settings.Language != "" {
		t.Fatal("CLI override leaked into saved settings")
	}
	a.setLanguage("")
	a.bgWG.Wait()
	tt.Frame()
	if a.lang() != LangZH {
		t.Fatal("explicit system selection did not clear the process override")
	}
}
