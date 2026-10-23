package launcher

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
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
		return CrashInsight{Kind: "memory", Cause: "内存不足，游戏用完了分配的内存", CauseEn: "Out of memory: the game used up its heap",
			Advice: "在版本设置里调高内存，或减少高耗内存的模组和光影", AdviceEn: "Raise memory in version settings, or drop heavy mods and shaders"}
	case has("UnsupportedClassVersionError", "has been compiled by a more recent version"):
		need := javaForClass(first(reClassVer, 1))
		c := CrashInsight{Kind: "java", Cause: "Java 版本过低，模组或游戏需要更新的 Java", CauseEn: "Java is too old for this game or a mod",
			Advice: "在版本设置里换用更新的 Java", AdviceEn: "Pick a newer Java in version settings"}
		if need != "" {
			c.Advice, c.AdviceEn = "在版本设置里换用 Java "+need+" 或更高", "Pick Java "+need+" or newer in version settings"
		}
		return c
	case has("Missing or unsupported mandatory dependencies", "missing mandatory dependencies", "requires any version of", "Unmet dependency", "Incompatible mods found"):
		c := CrashInsight{Kind: "dependency", Cause: "缺少前置模组，或前置版本不对", CauseEn: "A required mod is missing or the wrong version",
			Advice: "按提示安装缺少的前置模组，或换用匹配的版本", AdviceEn: "Install the missing dependency, or a matching version"}
		if m := reNeedMod.FindStringSubmatch(text); m != nil {
			c.Mod = m[2]
			c.Advice, c.AdviceEn = m[2]+" 需要前置 "+m[1]+"，请安装后再启动", m[2]+" needs "+m[1]+"; install it and launch again"
		} else if m := reNeedFab.FindStringSubmatch(text); m != nil {
			c.Mod = m[1]
			c.Advice, c.AdviceEn = m[1]+" 需要前置 "+m[3]+"，请安装后再启动", m[1]+" needs "+m[3]+"; install it and launch again"
		}
		return c
	case reDupMod.MatchString(text):
		return CrashInsight{Kind: "duplicate", Cause: "装了重复的模组", CauseEn: "The same mod is installed twice",
			Advice: "在模组页删去重复的那个", AdviceEn: "Remove the duplicate on the Mods page"}
	case has("Mixin apply", "MixinApplyError", "InvalidMixinException", "MixinTransformerError", "InvalidInjectionException"):
		c := CrashInsight{Kind: "mixin", Cause: "模组冲突，某个模组的 Mixin 注入失败", CauseEn: "Mod conflict: a mod's mixin failed to apply",
			Advice: "先禁用下方模组再试，或把它和相关模组更新到匹配版本", AdviceEn: "Disable the mod below and retry, or update it and related mods"}
		if m := first(reMixinMod, 1); m != "" {
			c.Mod = m
		} else if m := first(reMixinCfg, 1); m != "" {
			c.Mod = m
		}
		return c
	case has("Failed to create GL context", "Pixel format not accelerated", "GLFW error 65542", "WGL: The driver does not appear to support OpenGL", "EXCEPTION_ACCESS_VIOLATION"):
		c := CrashInsight{Kind: "graphics", Cause: "显卡驱动或图形相关模组出错", CauseEn: "Graphics driver or a rendering mod failed",
			Advice: "更新显卡驱动，或先关掉光影和渲染类模组", AdviceEn: "Update the graphics driver, or turn off shaders and render mods"}
		if has("EXCEPTION_ACCESS_VIOLATION") {
			c.Cause, c.CauseEn = "游戏进程内存访问出错，常见于显卡驱动或原生模组", "Native crash, often a graphics driver or native mod"
		}
		return c
	case has("ModLoadingException", "Caught exception from", "LoadingFailedException", "Mod File:", "Suspected Mod"):
		c := CrashInsight{Kind: "mod", Cause: "模组加载时出错", CauseEn: "A mod failed while loading",
			Advice: "更新或禁用下方模组后再试", AdviceEn: "Update or disable the mod below and retry"}
		if m := reCaughtMod.FindStringSubmatch(text); m != nil {
			c.Mod = strings.TrimSpace(m[1])
		} else if m := first(reSuspect, 1); m != "" && !strings.EqualFold(m, "NONE") && !strings.HasPrefix(m, "Unknown") {
			c.Mod = m
		} else if m := first(reModFile, 1); m != "" {
			c.Mod = m
		}
		return c
	}
	return CrashInsight{Kind: "unknown", Cause: "游戏异常退出，没有找到已知原因", CauseEn: "The game quit unexpectedly; no known cause found",
		Advice: "查看崩溃报告或日志末尾的错误", AdviceEn: "Check the crash report or the errors at the end of the log"}
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
		in.Kind = "killed"
		in.Cause, in.CauseEn = "游戏进程被外部结束", "The game process was ended from outside"
		in.Advice, in.AdviceEn = "可能被任务管理器或其他程序结束，可直接重新启动", "Task Manager or another program may have ended it; just launch again"
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
