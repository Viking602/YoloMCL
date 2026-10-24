package app

import "testing"

func TestLauncherArgumentsSurviveEntryPointMove(t *testing.T) {
	o, err := parseOptions([]string{"-render", "page.png", "-page", "settings", "-lang", "zh-CN", "-width", "900", "-height", "600", "-diag", "report.json"})
	if err != nil || o.render != "page.png" || o.page != "settings" || o.language != "zh-CN" || o.width != 900 || o.height != 600 || o.diagnostic != "report.json" {
		t.Fatalf("launcher arguments changed: %+v, %v", o, err)
	}
	defaults, err := parseOptions(nil)
	if err != nil || defaults.page != "home" || defaults.width != 1536 || defaults.height != 1024 {
		t.Fatalf("default window settings changed: %+v, %v", defaults, err)
	}
	if err := Run([]string{"-help"}); err != nil {
		t.Fatal("help must return without starting MyGo", err)
	}
	if err := Run([]string{"-unknown"}); err == nil {
		t.Fatal("invalid options must fail before starting MyGo")
	}
}
