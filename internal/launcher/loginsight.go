package launcher

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

// CrashInsight is a plain-language guess at why a game crashed, from known
// patterns in its log and crash report. Mod is the culprit when one is named.
type CrashInsight struct {
	Kind     string `json:"kind"`
	Cause    string `json:"cause"`
	CauseEn  string `json:"causeEn"`
	Advice   string `json:"advice"`
	AdviceEn string `json:"adviceEn"`
	Mod      string `json:"mod,omitempty"`
}

func crashText(key string, args ...any) (zh, en string) {
	zh, en = i18n.Text(false, key), i18n.Text(true, key)
	if len(args) > 0 {
		zh, en = fmt.Sprintf(zh, args...), fmt.Sprintf(en, args...)
	}
	return zh, en
}

func newCrashInsight(kind string) CrashInsight {
	c := CrashInsight{Kind: kind}
	c.Cause, c.CauseEn = crashText("crash." + kind + ".cause")
	c.Advice, c.AdviceEn = crashText("crash." + kind + ".advice")
	return c
}

var (
	reMixinCfg  = regexp.MustCompile(`(?i)mixin config ([a-z0-9_.\-]+?)\.mixins?\.json`)
	reMixinMod  = regexp.MustCompile(`(?i)from mod ([a-z0-9_\-]+)`)
	reCaughtMod = regexp.MustCompile(`(?i)Caught exception from ([^\n(]+?)\s*\(([a-z0-9_\-]+)\)`)
	reModFile   = regexp.MustCompile(`(?i)Mod File: (?:.*[/\\])?([^/\\\n]+\.jar)`)
	reSuspect   = regexp.MustCompile(`(?i)Suspected Mods?: ([^\n]+)`)
	reNeedMod   = regexp.MustCompile(`(?i)Mod ID: '([^']+)', Requested by: '([^']+)'`)
	reNeedFab   = regexp.MustCompile(`(?i)Mod '([^']+)' \(([a-z0-9_\-]+)\) [^\n]*? requires [^\n]*?'([^']+)'`)
	reClassVer  = regexp.MustCompile(`class file version (\d+)\.0`)
	reDupMod    = regexp.MustCompile(`(?i)(?:Duplicate mods? found|DuplicateModsFoundException|found duplicate mods?)[^\n]*`)
)

// javaForClass maps a class-file version to the Java release that reads it.
func javaForClass(v string) string {
	n := 0
	for _, c := range v {
		n = n*10 + int(c-'0')
	}
	if n >= 45 {
		return itoa(n - 44)
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// DiagnoseCrash matches the most specific known pattern first.
func DiagnoseCrash(text string) CrashInsight {
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(text, s) {
				return true
			}
		}
		return false
	}
	first := func(re *regexp.Regexp, g int) string {
		if m := re.FindStringSubmatch(text); m != nil {
			return strings.TrimSpace(m[g])
		}
		return ""
	}
	switch {
	case has("java.lang.OutOfMemoryError", "Out of memory", "GC overhead limit exceeded"):
		return newCrashInsight("memory")
	case has("UnsupportedClassVersionError", "has been compiled by a more recent version"):
		need := javaForClass(first(reClassVer, 1))
		c := newCrashInsight("java")
		if need != "" {
			c.Advice, c.AdviceEn = crashText("crash.java.versionAdvice", need)
		}
		return c
	case has("Missing or unsupported mandatory dependencies", "missing mandatory dependencies", "requires any version of", "Unmet dependency", "Incompatible mods found"):
		c := newCrashInsight("dependency")
		if m := reNeedMod.FindStringSubmatch(text); m != nil {
			c.Mod = m[2]
			c.Advice, c.AdviceEn = crashText("crash.dependency.modAdvice", m[2], m[1])
		} else if m := reNeedFab.FindStringSubmatch(text); m != nil {
			c.Mod = m[1]
			c.Advice, c.AdviceEn = crashText("crash.dependency.modAdvice", m[1], m[3])
		}
		return c
	case reDupMod.MatchString(text):
		return newCrashInsight("duplicate")
	case has("Mixin apply", "MixinApplyError", "InvalidMixinException", "MixinTransformerError", "InvalidInjectionException"):
		c := newCrashInsight("mixin")
		if m := first(reMixinMod, 1); m != "" {
			c.Mod = m
		} else if m := first(reMixinCfg, 1); m != "" {
			c.Mod = m
		}
		return c
	case has("Failed to create GL context", "Pixel format not accelerated", "GLFW error 65542", "WGL: The driver does not appear to support OpenGL", "EXCEPTION_ACCESS_VIOLATION"):
		c := newCrashInsight("graphics")
		if has("EXCEPTION_ACCESS_VIOLATION") {
			c.Cause, c.CauseEn = crashText("crash.graphics.nativeCause")
		}
		return c
	case has("ModLoadingException", "Caught exception from", "LoadingFailedException", "Mod File:", "Suspected Mod"):
		c := newCrashInsight("mod")
		if m := reCaughtMod.FindStringSubmatch(text); m != nil {
			c.Mod = strings.TrimSpace(m[1])
		} else if m := first(reSuspect, 1); m != "" && !strings.EqualFold(m, "NONE") && !strings.HasPrefix(m, "Unknown") {
			c.Mod = m
		} else if m := first(reModFile, 1); m != "" {
			c.Mod = m
		}
		return c
	}
	return newCrashInsight("unknown")
}

// findCrashReport is the newest crash report or JVM error log written in
// gameDir since the session started.
func findCrashReport(gameDir string, since time.Time) string {
	var best string
	var bt time.Time
	for _, pat := range []string{filepath.Join(gameDir, "crash-reports", "crash-*.txt"), filepath.Join(gameDir, "hs_err_pid*.log")} {
		ms, _ := filepath.Glob(pat)
		for _, m := range ms {
			if st, e := os.Stat(m); e == nil && !st.ModTime().Before(since.Add(-2*time.Second)) && st.ModTime().After(bt) {
				best, bt = m, st.ModTime()
			}
		}
	}
	return best
}

// readHead reads at most n bytes of a file.
func readHead(p string, n int64) string {
	f, e := os.Open(p)
	if e != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, n))
	return string(b)
}

// diagnoseSession runs on crash: the crash report wins, the log tail helps.
func diagnoseSession(gs *gameSession) {
	if gs.GameDir != "" {
		gs.CrashReport = findCrashReport(gs.GameDir, gs.Started)
	}
	var b strings.Builder
	if gs.CrashReport != "" {
		b.WriteString(readHead(gs.CrashReport, 512<<10))
		b.WriteByte('\n')
	}
	if gs.log != nil {
		lines, _ := gs.log.snapshot()
		if len(lines) > 3000 {
			lines = lines[len(lines)-3000:]
		}
		b.WriteString(strings.Join(lines, "\n"))
	}
	in := DiagnoseCrash(b.String())
	if gs.ExitCode == -1 && gs.CrashReport == "" && in.Kind != "memory" { // TerminateProcess, not a crash
		killed := newCrashInsight("killed")
		killed.Mod = in.Mod
		in = killed
	}
	gs.Insight = &in
}

// LogFile is one file under a version's logs or crash-reports folder.
type LogFile struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Kind    string    `json:"kind"` // log | crash
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// RecentLogFiles lists the newest logs and crash reports of one version.
func (s *Service) RecentLogFiles(rootID, versionID string, limit int) ([]LogFile, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil, e
	}
	dir := r.VersionDir()
	var out []LogFile
	add := func(pat, kind string) {
		ms, _ := filepath.Glob(filepath.Join(dir, pat))
		for _, m := range ms {
			if st, e := os.Stat(m); e == nil && st.Mode().IsRegular() && st.Size() > 0 {
				out = append(out, LogFile{Path: m, Name: filepath.Base(m), Kind: kind, Size: st.Size(), ModTime: st.ModTime()})
			}
		}
	}
	add(filepath.Join("logs", "*.log"), "log")
	add(filepath.Join("crash-reports", "*.txt"), "crash")
	add("hs_err_pid*.log", "crash")
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// insideVersion keeps file reads to the version's own logs and reports.
func (s *Service) insideVersion(rootID, versionID, p string) error {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(r.VersionDir(), filepath.Clean(p))
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("只能读取这个版本的日志")
	}
	return nil
}

// FileTail pages a chosen log file of one version, like LogTail.
func (s *Service) FileTail(rootID, versionID, p string, n int, before int64) (LogChunk, error) {
	if e := s.insideVersion(rootID, versionID, p); e != nil {
		return LogChunk{}, e
	}
	lines, start, e := TailLines(p, n, before)
	return LogChunk{Lines: lines, Start: start, Path: p}, e
}

// CrashReportText is the start of a crash report, for copying (1 MB max).
func (s *Service) CrashReportText(rootID, versionID, p string) (string, error) {
	if e := s.insideVersion(rootID, versionID, p); e != nil {
		return "", e
	}
	t := readHead(p, 1<<20)
	if t == "" {
		return "", errors.New("崩溃报告是空的")
	}
	return t, nil
}
