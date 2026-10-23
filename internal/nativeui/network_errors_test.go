package nativeui

import (
	"errors"
	"strings"
	"testing"
)

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
