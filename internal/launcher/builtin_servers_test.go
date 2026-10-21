package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinAuthServersFirstAndDeduped(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	// legacy saved duplicates (trailing slash, http, site root, resolved API URL) + a custom one
	s.mu.Lock()
	s.cfg.AuthServers = []AuthServer{
		{Name: "My LS", URL: "https://LittleSkin.cn/api/yggdrasil/"},
		{Name: "ely old", URL: "https://authserver.ely.by"},
		{Name: "Custom", URL: "https://skin.example/api/yggdrasil"},
	}
	s.cfg.Accounts = append(s.cfg.Accounts,
		Account{ID: "y1", Name: "A", UUID: "0123456789abcdef0123456789abcdef", Kind: "yggdrasil", Server: "http://littleskin.cn"},
		Account{ID: "y2", Name: "B", UUID: "1123456789abcdef0123456789abcdef", Kind: "yggdrasil", Server: "https://account.ely.by/api/authlib-injector/"})
	s.mu.Unlock()
	got := s.Snapshot().AuthServers
	if len(got) != 3 || got[0].Name != "LittleSkin" || !got[0].Builtin || got[0].URL != "https://littleskin.cn/api/yggdrasil" ||
		got[1].Name != "Ely.by" || !got[1].Builtin || got[1].URL != "https://account.ely.by/api/authlib-injector" || got[2].Name != "Custom" || got[2].Builtin {
		t.Fatalf("list: %+v", got)
	}
	// accounts stay linked to the built-in
	for _, a := range s.Snapshot().Accounts {
		if a.Kind == "yggdrasil" {
			if _, ok := BuiltinAuthServer(a.Server); !ok {
				t.Errorf("%s not linked to a built-in", a.Server)
			}
		}
	}
	if !SameAuthServer("https://ely.by/", "https://account.ely.by/api/authlib-injector") || SameAuthServer("https://littleskin.cn", "https://skin.example/api/yggdrasil") {
		t.Error("SameAuthServer")
	}
}

func TestBuiltinAuthServersFixedAndNotStored(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e := s.DeleteAuthServer("https://littleskin.cn/api/yggdrasil"); e == nil {
		t.Error("built-in must not be deletable")
	}
	if _, e := s.SaveAuthServer("https://account.ely.by/api/authlib-injector", "Renamed", "https://x.example/api"); e == nil {
		t.Error("built-in must not be editable")
	}
	if v, e := s.SaveAuthServer("", "Mine", "https://littleskin.cn/"); e != nil || v.Name != "LittleSkin" || !v.Builtin {
		t.Errorf("saving a built-in URL: %+v %v", v, e)
	}
	s.rememberAuthServer(t.Context(), "https://littleskin.cn/api/yggdrasil")
	if _, e := s.SaveAuthServer("", "Custom", "https://skin.example/api"); e != nil {
		t.Fatal(e)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, m := range matches {
		b, _ := os.ReadFile(m)
		if strings.Contains(strings.ToLower(string(b)), "littleskin") || strings.Contains(string(b), "ely.by") || strings.Contains(string(b), "builtin") {
			t.Errorf("%s polluted with built-ins: %s", m, b)
		}
	}
	if len(s.cfg.AuthServers) != 1 {
		t.Errorf("config servers: %+v", s.cfg.AuthServers)
	}
}
