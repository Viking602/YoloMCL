package nativeui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// uiError is what the UI shows for a failure: a plain sentence, what to do
// about it, and (when known) the folder involved. The raw error only goes
// to the log.
type uiError struct {
	Kind, Title, Hint, Path string
}

func (u uiError) Text() string {
	if u.Hint == "" {
		return u.Title
	}
	return u.Title + "\n" + u.Hint
}

var (
	errPathRe = regexp.MustCompile(`(?i)(?:open|stat|lstat|mkdir|remove|rename|readdir|read|write|create)\s+((?:[a-z]:\\|/)[^:]*?)(?::|$)`)
	hostRe    = regexp.MustCompile(`(?i)\b[a-z0-9-]+(\.[a-z0-9-]+)*\.(com|net|org|io|dev|cn|gg|me|app|cc)\b`)
)

// looksNetworkError reports whether raw error text came from a remote request.
func looksNetworkError(low string) bool {
	if hostRe.MatchString(low) {
		return true
	}
	for _, k := range []string{"http", "://", "no such host", "dial tcp", "connection refused", "connection reset", "network", "tls:", "x509:", "eof", "unexpected status", "status code", "status 4", "status 5", "proxy", "dns", "i/o timeout", "timeout", "timed out", "deadline exceeded", "too many requests", "unreachable", "read tcp", "write tcp", "wsarecv", "wsasend", "forcibly closed"} {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// classifyError maps any error text to a friendly message in zh or en.
func classifyError(en bool, raw string, e error) uiError {
	L := func(zh, eng string) string {
		if en {
			return eng
		}
		return zh
	}
	low := strings.ToLower(raw)
	path := ""
	if m := errPathRe.FindStringSubmatch(raw); m != nil {
		path = strings.TrimSpace(m[1])
	}
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(low, s) {
				return true
			}
		}
		return false
	}
	var ne net.Error
	switch {
	case e != nil && errors.Is(e, fs.ErrPermission), has("access is denied", "permission denied", "operation not permitted"):
		return uiError{"denied", L("没有权限访问这个文件夹", "No permission to access this folder"), L("关闭可能占用它的程序后重试；如果仍然不行，请以管理员身份运行启动器，或检查文件夹的安全设置。", "Close any program that may be using it and retry. If that does not help, run the launcher as administrator or check the folder's security settings."), path}
	case has("being used by another process", "sharing violation", "used by another process"):
		return uiError{"busy", L("文件正被其他程序占用", "The file is in use by another program"), L("关闭游戏或其他占用它的程序后重试。", "Close the game or other program using it, then retry."), path}
	case has("not enough space", "no space left", "disk full", "there is not enough space"):
		return uiError{"disk", L("磁盘空间不足", "The disk is full"), L("清理磁盘空间后重试。", "Free up some disk space and retry."), path}
	case e != nil && (errors.Is(e, context.DeadlineExceeded) || errors.As(e, &ne)), looksNetworkError(low):
		// Remote failures say only this; URLs, hosts and codes stay in the log.
		return uiError{"network", L("加载失败", "Failed to load"), "", ""}
	case e != nil && errors.Is(e, fs.ErrNotExist), has("cannot find the path", "cannot find the file", "no such file", "not exist", "system cannot find"):
		return uiError{"missing", L("找不到需要的文件或文件夹", "A needed file or folder is missing"), L("它可能被移动或删除了。重新扫描实例后重试。", "It may have been moved or deleted. Rescan the instance and retry."), path}
	}
	if friendlyAlready(raw, en) {
		return uiError{"plain", raw, "", ""}
	}
	return uiError{"other", L("操作没有完成", "Something went wrong"), L("请重试。详细信息已写入日志。", "Please retry. Details were written to the log."), path}
}

// friendlyAlready reports whether a message was written for people
// (the launcher's own localized errors) rather than by the runtime.
func friendlyAlready(s string, en bool) bool {
	if s == "" || len(s) > 240 || strings.ContainsAny(s, "\\{}") || strings.Contains(s, "0x") {
		return false
	}
	for _, tok := range []string{"go:", "panic", "runtime", "errno", "syscall", ": open ", "json:", "invalid character", "unexpected end", "exit status", "nil pointer", "://"} {
		if strings.Contains(strings.ToLower(s), tok) {
			return false
		}
	}
	cjk := false
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			cjk = true
			break
		}
	}
	if !en {
		return cjk
	}
	return !cjk && strings.Count(s, ":") <= 1
}

var errLogMu sync.Mutex

func logUIError(raw string) {
	dir, e := os.UserConfigDir()
	if e != nil {
		return
	}
	dir = filepath.Join(dir, "YoloMCL", "logs")
	_ = os.MkdirAll(dir, 0o755)
	errLogMu.Lock()
	defer errLogMu.Unlock()
	f, e := os.OpenFile(filepath.Join(dir, "ui-errors.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), raw)
}

// uiErr is the friendly error for e, logging the raw text.
func (a *App) uiErr(e error) uiError {
	if e == nil {
		return uiError{}
	}
	raw := e.Error()
	logUIError(raw)
	return classifyError(a.isEN(), raw, e)
}

// friendlyErr is the one string the UI may show for an error.
func (a *App) friendlyErr(e error) string { return a.uiErr(e).Text() }
