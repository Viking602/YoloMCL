package launcher

import "testing"

func TestSystemLanguagePreferenceOrder(t *testing.T) {
	for _, tc := range []struct {
		name      string
		preferred []string
		want      string
	}{
		{"Windows Chinese", []string{"zh-CN", "en-US"}, "zh-CN"},
		{"Windows English", []string{"en-GB", "zh-CN"}, "en-US"},
		{"macOS script tag", []string{"zh-Hans-CN", "en-US"}, "zh-CN"},
		{"Chinese variant", []string{"zh-Hant-HK", "en-GB"}, "zh-CN"},
		{"secondary supported preference", []string{"fr-FR", "zh-Hans", "en"}, "zh-CN"},
		{"Linux UTF8", []string{"zh_CN.UTF-8", "en_US"}, "zh-CN"},
		{"Linux modifier", []string{"en_GB.UTF-8@euro"}, "en-US"},
		{"unsupported", []string{"ja-JP", "de-DE"}, "en-US"},
		{"no locale", nil, "en-US"},
		{"POSIX", []string{"C", "C.UTF-8", "POSIX"}, "en-US"},
		{"not a language prefix", []string{"english", "zhongwen", "enochian"}, "en-US"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SystemLanguage(tc.preferred); got != tc.want {
				t.Fatalf("%v resolved to %q, want %q", tc.preferred, got, tc.want)
			}
		})
	}
}

func TestNormalizeLanguageMode(t *testing.T) {
	for input, want := range map[string]string{
		"": "", "auto": "", "SYSTEM": "", "fr-FR": "", "english": "",
		"zh-CN": "zh-CN", "zh_Hant_TW.UTF-8": "zh-CN", " EN-us ": "en-US",
	} {
		if got := NormalizeLanguage(input); got != want {
			t.Errorf("NormalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAutomaticLanguageModeSurvivesReload(t *testing.T) {
	t.Setenv("MCL_LANG", "zh-CN")
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	settings := s.Snapshot().Settings
	settings.Language = "system"
	if err = s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot().Settings.Language; got != "" {
		t.Fatalf("automatic mode replaced by environment default: %q", got)
	}
}
