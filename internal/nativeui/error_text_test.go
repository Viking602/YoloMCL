package nativeui

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestClassifyErrorNeverRaw(t *testing.T) {
	cases := map[string]string{
		`open D:\MC\.minecraft\versions\X\mods: Access is denied.`:                        "denied",
		`open /x/mods: permission denied`:                                                 "denied",
		`The process cannot access the file because it is being used by another process.`: "busy",
		`write C:\a\b.jar: There is not enough space on the disk.`:                        "disk",
		`Get "https://api.modrinth.com/v2": context deadline exceeded`:                    "network",
		`dial tcp: lookup api.curseforge.com: no such host`:                               "network",
		`open C:\nope\x.json: The system cannot find the path specified.`:                 "missing",
		`json: cannot unmarshal string into Go value of type int`:                         "other",
	}
	for raw, kind := range cases {
		for _, en := range []bool{false, true} {
			u := classifyError(en, raw, nil)
			if u.Kind != kind {
				t.Errorf("%q en=%v: kind %s, want %s", raw, en, u.Kind, kind)
			}
			txt := u.Text()
			if strings.Contains(txt, raw) || strings.Contains(strings.ToLower(txt), "access is denied") || strings.Contains(txt, "json:") || strings.Contains(txt, `D:\`) {
				t.Errorf("raw text leaked: %q", txt)
			}
		}
	}
	if u := classifyError(false, `open D:\MC\mods: Access is denied.`, nil); u.Path != `D:\MC\mods` {
		t.Errorf("path = %q", u.Path)
	}
	if u := classifyError(false, "", &fs.PathError{Op: "open", Path: "x", Err: fs.ErrPermission}); u.Kind != "denied" {
		t.Errorf("typed permission error: %s", u.Kind)
	}
	if u := classifyError(false, "版本 1.0 没有可下载文件", errors.New("x")); u.Kind != "plain" {
		t.Errorf("launcher's own Chinese message should pass through: %s", u.Kind)
	}
}

func TestNetworkErrorsHideURLs(t *testing.T) {
	raws := []string{
		`Get "https://api.modrinth.com/v2/search?query=x": dial tcp: lookup api.modrinth.com: no such host`,
		`Post "https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode": context deadline exceeded`,
		`unexpected status 503 from https://api.curseforge.com/v1/mods/search`,
		`HTTP 404`,
		`read tcp 10.0.0.2:5123->104.18.1.1:443: wsarecv: An existing connection was forcibly closed`,
		`textures.minecraft.net: i/o timeout`,
		`x509: certificate signed by unknown authority`,
	}
	for _, en := range []bool{false, true} {
		for _, r := range raws {
			u := classifyError(en, r, errors.New(r))
			txt := strings.ToLower(u.Text() + u.Path)
			if strings.Contains(txt, "http") || hostRe.MatchString(txt) || strings.ContainsAny(txt, "0123456789") {
				t.Errorf("leaks detail: %q -> %q", r, txt)
			}
			want := "加载失败"
			if en {
				want = "Failed to load"
			}
			if u.Title != want {
				t.Errorf("%q -> %q, want %q", r, u.Title, want)
			}
		}
	}
}
