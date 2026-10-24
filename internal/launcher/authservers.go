package launcher

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// NormalizeAuthServerURL trims whitespace and trailing slashes and requires
// an HTTPS URL without credentials (same rule as third-party login).
func NormalizeAuthServerURL(raw string) (string, error) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if e := validateURL(v); e != nil {
		return "", e
	}
	return v, nil
}

func authServerHost(raw string) string {
	if u, e := url.Parse(raw); e == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// BuiltinAuthServers are always offered first (API roots verified via the
// X-Authlib-Injector-API-Location header and their meta endpoints).
var BuiltinAuthServers = []AuthServer{
	{Name: "LittleSkin", URL: "https://littleskin.cn/api/yggdrasil", Builtin: true},
	{Name: "Ely.by", URL: "https://account.ely.by/api/authlib-injector", Builtin: true},
}

// builtinAliases maps scheme-less, lower-case addresses users may have
// typed (site root, API root, or the resolved API URL) to a built-in.
var builtinAliases = map[string]int{
	"littleskin.cn": 0, "www.littleskin.cn": 0, "littleskin.cn/api/yggdrasil": 0,
	"ely.by": 1, "www.ely.by": 1, "account.ely.by": 1, "account.ely.by/api/authlib-injector": 1,
	"authserver.ely.by": 1, "authserver.ely.by/api/authlib-injector": 1,
}

// authServerKey normalises a server address for comparison: lower case,
// no scheme, no trailing slash.
func authServerKey(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	v = strings.TrimPrefix(strings.TrimPrefix(v, "https://"), "http://")
	return strings.TrimRight(v, "/")
}

// BuiltinAuthServer returns the built-in server an address refers to.
func BuiltinAuthServer(raw string) (AuthServer, bool) {
	if i, ok := builtinAliases[authServerKey(raw)]; ok {
		return BuiltinAuthServers[i], true
	}
	return AuthServer{}, false
}

// SameAuthServer reports whether two addresses denote the same server.
func SameAuthServer(a, b string) bool {
	if x, ok := BuiltinAuthServer(a); ok {
		y, ok2 := BuiltinAuthServer(b)
		return ok2 && x.URL == y.URL
	}
	return authServerKey(a) == authServerKey(b)
}

// authServersLocked returns built-ins, then saved servers, plus (for
// backward compatibility) servers of third-party accounts never saved.
// Duplicates of built-ins are shown once, as the built-in.
func (s *Service) authServersLocked() []AuthServer {
	out := append([]AuthServer{}, BuiltinAuthServers...)
	seen := map[string]bool{}
	for _, v := range out {
		seen[authServerKey(v.URL)] = true
	}
	add := func(v AuthServer) {
		if _, b := BuiltinAuthServer(v.URL); b || seen[authServerKey(v.URL)] {
			return
		}
		seen[authServerKey(v.URL)] = true
		out = append(out, v)
	}
	for _, v := range s.cfg.AuthServers {
		v.Builtin = false
		add(v)
	}
	for _, a := range s.cfg.Accounts {
		if a.Kind == "yggdrasil" && a.Server != "" {
			add(AuthServer{Name: authServerHost(a.Server), URL: a.Server})
		}
	}
	return out
}

// SaveAuthServer adds or updates (matched by previous URL, or by URL) a
// saved server. An empty name defaults to the host.
func (s *Service) SaveAuthServer(previousURL, name, raw string) (AuthServer, error) {
	u, e := NormalizeAuthServerURL(raw)
	if e != nil {
		return AuthServer{}, e
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = authServerHost(u)
	}
	if len([]rune(name)) > 64 {
		return AuthServer{}, errors.New("服务器名称过长")
	}
	if b, ok := BuiltinAuthServer(u); ok {
		return b, nil // built-ins are fixed and never stored
	}
	if _, ok := BuiltinAuthServer(previousURL); ok {
		return AuthServer{}, errors.New("内置服务器不可编辑")
	}
	v := AuthServer{Name: name, URL: u}
	prev := strings.ToLower(strings.TrimRight(strings.TrimSpace(previousURL), "/"))
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	list := draft.AuthServers[:0:0]
	replaced := false
	for _, x := range draft.AuthServers {
		k := strings.ToLower(x.URL)
		if k == prev || k == strings.ToLower(u) {
			if !replaced {
				list = append(list, v)
				replaced = true
			}
			continue
		}
		list = append(list, x)
	}
	if !replaced {
		list = append(list, v)
	}
	draft.AuthServers = list
	return v, s.saveConfigLocked(draft)
}

// DeleteAuthServer removes a saved server entry only; accounts that use it
// are untouched (and keep the server listed via authServersLocked).
func (s *Service) DeleteAuthServer(raw string) error {
	if _, ok := BuiltinAuthServer(raw); ok {
		return errors.New("内置服务器不可删除")
	}
	k := strings.ToLower(strings.TrimRight(strings.TrimSpace(raw), "/"))
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	list := draft.AuthServers[:0:0]
	for _, x := range draft.AuthServers {
		if strings.ToLower(x.URL) != k {
			list = append(list, x)
		}
	}
	draft.AuthServers = list
	return s.saveConfigLocked(draft)
}

// FetchAuthServerName reads meta.serverName from the Yggdrasil API root.
func (s *Service) FetchAuthServerName(ctx context.Context, raw string) (string, error) {
	u, e := NormalizeAuthServerURL(raw)
	if e != nil {
		return "", e
	}
	var meta struct {
		Meta struct {
			ServerName string `json:"serverName"`
		} `json:"meta"`
	}
	if e := s.jsonRequest(ctx, "GET", u, nil, nil, &meta); e != nil {
		return "", e
	}
	if n := strings.TrimSpace(meta.Meta.ServerName); n != "" {
		return n, nil
	}
	return authServerHost(u), nil
}

// rememberAuthServer saves a server after a successful login if it is not
// listed yet (keeps the user-given name when present).
func (s *Service) rememberAuthServer(ctx context.Context, raw string) {
	if _, ok := BuiltinAuthServer(raw); ok {
		return
	}
	s.mu.Lock()
	for _, x := range s.cfg.AuthServers {
		if strings.EqualFold(x.URL, raw) {
			s.mu.Unlock()
			return
		}
	}
	s.mu.Unlock()
	name, _ := s.FetchAuthServerName(ctx, raw)
	_, _ = s.SaveAuthServer("", name, raw)
}
