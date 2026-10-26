package nativeui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/i18n"
	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestEveryJobTitleKeyTranslated(t *testing.T) {
	for _, k := range launcher.JobTitleKeys() {
		zh, hasZH := i18n.Lookup(false, "job.title."+k)
		en, hasEN := i18n.Lookup(true, "job.title."+k)
		if !hasZH || !hasEN || zh == "" || en == "" {
			t.Errorf("job title key %q has no zh/en translation", k)
		}
		if han.MatchString(en) {
			t.Errorf("en text for %q contains CJK: %q", k, en)
		}
	}
}

// A task created while the UI is Chinese re-renders in English after a live
// language switch (titles are keys rendered at display time).
func TestJobTitleFollowsLiveLanguageSwitch(t *testing.T) {
	a := &App{}
	j := launcher.Job{Title: "更新整合包：Fabulously Optimized", State: "running", Message: "下载新版本文件 (15/71): lithium.jar"}
	j.TitleKey, j.TitleArg, _ = launcher.JobTitleKey(j.Title)
	a.settings.Language = LangZH
	if got := a.jobTitle(j); got != "更新 Fabulously Optimized" {
		t.Fatalf("zh title %q", got)
	}
	a.settings.Language = LangEN
	if got := a.jobTitle(j); got != "Update Fabulously Optimized" {
		t.Fatalf("en title %q", got)
	}
	if got := jobStep(a.jobMsg(j.Message)); got != "Downloading new files" {
		t.Fatalf("en step %q", got)
	}
	for _, title := range []string{"下载整合包：X", "回滚整合包：X", "启动 X", "下载 Java 21", "安装 Minecraft 1.21.1", "备份全部存档", "安装光影包"} {
		jj := launcher.Job{Title: title}
		if han.MatchString(a.jobTitle(jj)) {
			t.Errorf("en render of %q still has CJK: %q", title, a.jobTitle(jj))
		}
	}
}

var han = regexp.MustCompile(`[\p{Han}]`)

// No hardcoded CJK in UI code outside the reviewed baseline: new Chinese text
// belongs in the JSON catalogs. The remaining baseline covers legacy input
// matching, diagnostics and stable accessibility/test identifiers.
func TestNoNewHardcodedCJK(t *testing.T) {
	got := cjkSites(t)
	b, _ := os.ReadFile(filepath.Join("testdata", "cjk_baseline.txt"))
	base := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			base[l] = true
		}
	}
	if os.Getenv("UPDATE_CJK_BASELINE") == "1" {
		_ = os.MkdirAll("testdata", 0o755)
		_ = os.WriteFile(filepath.Join("testdata", "cjk_baseline.txt"), []byte(strings.Join(got, "\n")+"\n"), 0o644)
		return
	}
	for _, g := range got {
		if !base[g] {
			t.Errorf("hardcoded CJK outside JSON catalogs: %s", g)
		}
	}
}

func cjkSites(t *testing.T) []string {
	lit := regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`")
	allowedBefore := regexp.MustCompile(`(\.Label\(\s*|\.Key\(\s*|c\.Key\([^)]*)$`)
	files, _ := filepath.Glob("*.go")
	set := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(b), "\n") {
			st := strings.TrimSpace(line)
			if strings.HasPrefix(st, "//") {
				continue
			}
			for _, m := range lit.FindAllStringIndex(line, -1) {
				s := line[m[0]:m[1]]
				if !han.MatchString(s) || allowedBefore.MatchString(line[:m[0]]) {
					continue
				}
				set[f+": "+s] = true
			}
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
