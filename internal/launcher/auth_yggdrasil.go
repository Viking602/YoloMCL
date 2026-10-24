package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type ThirdProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type thirdResponse struct {
	Access      string         `json:"accessToken"`
	ClientToken string         `json:"clientToken"`
	Selected    *ThirdProfile  `json:"selectedProfile"`
	Available   []ThirdProfile `json:"availableProfiles"`
}
type ThirdLoginResult struct {
	Profiles []ThirdProfile `json:"profiles"`
	Account  *Account       `json:"account,omitempty"`
}

func (s *Service) LoginThird(ctx context.Context, server, username, password, profileID string) (ThirdLoginResult, error) {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	if e := validateURL(server); e != nil {
		return ThirdLoginResult{}, e
	}
	if username == "" || password == "" {
		return ThirdLoginResult{}, errors.New("请输入账号和密码")
	}
	// Follow only the authlib-injector API-location header; never forward passwords to a different host.
	res, e := s.request(ctx, "GET", server, nil, nil)
	if e != nil {
		return ThirdLoginResult{}, e
	}
	api := res.Header.Get("X-Authlib-Injector-API-Location")
	res.Body.Close()
	if api != "" {
		base, _ := url.Parse(server)
		relative, e := url.Parse(api)
		if e != nil {
			return ThirdLoginResult{}, e
		}
		target := base.ResolveReference(relative)
		if target.Scheme != "https" || !strings.EqualFold(target.Host, base.Host) {
			return ThirdLoginResult{}, errors.New("认证站的 API 地址更换了域名，请直接填写其 HTTPS API 地址")
		}
		server = strings.TrimRight(target.String(), "/")
	}
	var r thirdResponse
	if e = s.jsonRequest(ctx, "POST", server+"/authserver/authenticate", map[string]any{"agent": map[string]any{"name": "Minecraft", "version": 1}, "username": username, "password": password, "clientToken": newID(), "requestUser": true}, nil, &r); e != nil {
		return ThirdLoginResult{}, fmt.Errorf("第三方登录失败，请检查认证地址和账号密码: %w", e)
	}
	if r.Access == "" {
		return ThirdLoginResult{}, errors.New("认证站未返回有效令牌")
	}
	selected := r.Selected
	if profileID != "" {
		selected = nil
		for _, p := range r.Available {
			if p.ID == profileID {
				p := p
				selected = &p
				break
			}
		}
		if selected == nil {
			return ThirdLoginResult{}, errors.New("所选角色不属于此账号")
		}
	}
	if selected == nil && len(r.Available) == 1 {
		selected = &r.Available[0]
	}
	if selected == nil {
		return ThirdLoginResult{Profiles: r.Available}, nil
	}
	if r.Selected == nil || r.Selected.ID != selected.ID {
		var next thirdResponse
		e = s.jsonRequest(ctx, "POST", server+"/authserver/refresh", map[string]any{"accessToken": r.Access, "clientToken": r.ClientToken, "selectedProfile": selected, "requestUser": true}, nil, &next)
		if e != nil {
			return ThirdLoginResult{}, e
		}
		if next.Access == "" {
			return ThirdLoginResult{}, errors.New("角色绑定失败")
		}
		r.Access = next.Access
		if next.ClientToken != "" {
			r.ClientToken = next.ClientToken
		}
	}
	if !validUUID(selected.ID) {
		return ThirdLoginResult{}, errors.New("认证站返回的角色 UUID 无效")
	}
	a := Account{ID: "yg-" + serverID(server) + "-" + selected.ID, Name: selected.Name, UUID: strings.ReplaceAll(selected.ID, "-", ""), Kind: "yggdrasil", Server: server}
	e = s.saveAccount(a, tokenSecret{Access: r.Access, ClientToken: r.ClientToken, Expires: time.Now().Add(12 * time.Hour)})
	if e == nil {
		mctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		s.rememberAuthServer(mctx, server)
		cancel()
	}
	return ThirdLoginResult{Account: &a, Profiles: []ThirdProfile{}}, e
}
func serverID(v string) string {
	b, _ := json.Marshal(v)
	var h uint32 = 2166136261
	for _, v := range b {
		h ^= uint32(v)
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

func (s *Service) refreshThird(ctx context.Context, a Account, t tokenSecret) (tokenSecret, error) {
	var next thirdResponse
	if e := s.jsonRequest(ctx, "POST", a.Server+"/authserver/refresh", map[string]any{"accessToken": t.Access, "clientToken": t.ClientToken, "selectedProfile": ThirdProfile{ID: a.UUID, Name: a.Name}, "requestUser": true}, nil, &next); e != nil {
		return t, errors.New("第三方登录已过期，请重新登录")
	}
	if next.Access == "" {
		return t, errors.New("第三方令牌刷新失败")
	}
	t.Access = next.Access
	if next.ClientToken != "" {
		t.ClientToken = next.ClientToken
	}
	t.Expires = time.Now().Add(12 * time.Hour)
	return t, s.putSecret(a.ID, t)
}
