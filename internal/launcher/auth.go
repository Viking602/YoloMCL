package launcher

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const microsoftBase = "https://login.microsoftonline.com/consumers/oauth2/v2.0"
const microsoftScope = "XboxLive.signin offline_access"

type tokenSecret struct {
	Access      string    `json:"access"`
	Refresh     string    `json:"refresh"`
	ClientToken string    `json:"clientToken"`
	Expires     time.Time `json:"expires"`
}
type deviceSession struct {
	Code     string
	ClientID string
	Expires  time.Time
	Interval time.Duration
	LastPoll time.Time
	Polling  bool
}
type DeviceInfo struct {
	ID              string `json:"id"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}
type LoginResult struct {
	Status  string   `json:"status"`
	Account *Account `json:"account,omitempty"`
}
type oauthToken struct {
	Access      string `json:"access_token"`
	Refresh     string `json:"refresh_token"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func (s *Service) oauthForm(ctx context.Context, path string, form url.Values, out any) error {
	req, e := http.NewRequestWithContext(ctx, "POST", microsoftBase+path, strings.NewReader(form.Encode()))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "MCL/"+appVersion)
	r, e := s.http.Do(req)
	if e != nil {
		return fmt.Errorf("微软登录网络请求失败: %w", e)
	}
	defer r.Body.Close()
	if r.StatusCode >= 500 {
		return fmt.Errorf("微软登录服务返回 HTTP %d", r.StatusCode)
	}
	return decodeJSON(io.LimitReader(r.Body, 1<<20), out)
}
func (s *Service) StartMicrosoft(ctx context.Context) (DeviceInfo, error) {
	id := s.distribution.MicrosoftClientID
	if id == "" {
		return DeviceInfo{}, errors.New("此开发构建的官方登录服务尚未启用，请使用离线账号或第三方账号")
	}
	var r struct {
		DeviceCode  string `json:"device_code"`
		UserCode    string `json:"user_code"`
		URI         string `json:"verification_uri"`
		ExpiresIn   int    `json:"expires_in"`
		Interval    int    `json:"interval"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if e := s.oauthForm(ctx, "/devicecode", url.Values{"client_id": {id}, "scope": {microsoftScope}}, &r); e != nil {
		return DeviceInfo{}, e
	}
	if r.Error != "" {
		return DeviceInfo{}, fmt.Errorf("微软授权失败 (%s): %s", r.Error, r.Description)
	}
	if r.DeviceCode == "" || r.ExpiresIn <= 0 {
		return DeviceInfo{}, errors.New("微软未返回有效的设备授权码")
	}
	if r.Interval < 5 {
		r.Interval = 5
	}
	key := newID()
	s.mu.Lock()
	for k, d := range s.devices {
		if time.Now().After(d.Expires) {
			delete(s.devices, k)
		}
	}
	s.devices[key] = &deviceSession{Code: r.DeviceCode, ClientID: id, Expires: time.Now().Add(time.Duration(r.ExpiresIn) * time.Second), Interval: time.Duration(r.Interval) * time.Second}
	s.mu.Unlock()
	return DeviceInfo{ID: key, UserCode: r.UserCode, VerificationURI: r.URI, ExpiresIn: r.ExpiresIn, Interval: r.Interval}, nil
}
func (s *Service) CancelMicrosoft(id string) { s.mu.Lock(); delete(s.devices, id); s.mu.Unlock() }
func (s *Service) PollMicrosoft(ctx context.Context, id string) (LoginResult, error) {
	s.mu.Lock()
	d := s.devices[id]
	if d == nil {
		s.mu.Unlock()
		return LoginResult{}, errors.New("登录会话已结束，请重新登录")
	}
	if time.Now().After(d.Expires) {
		delete(s.devices, id)
		s.mu.Unlock()
		return LoginResult{}, errors.New("设备码已过期，请重新登录")
	}
	if d.Polling || time.Since(d.LastPoll) < d.Interval {
		s.mu.Unlock()
		return LoginResult{Status: "pending"}, nil
	}
	d.Polling = true
	d.LastPoll = time.Now()
	s.mu.Unlock()
	defer func() { s.mu.Lock(); d.Polling = false; s.mu.Unlock() }()
	var t oauthToken
	if e := s.oauthForm(ctx, "/token", url.Values{"client_id": {d.ClientID}, "device_code": {d.Code}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &t); e != nil {
		return LoginResult{}, e
	}
	if t.Error == "authorization_pending" {
		return LoginResult{Status: "pending"}, nil
	}
	if t.Error == "slow_down" {
		s.mu.Lock()
		d.Interval += 5 * time.Second
		s.mu.Unlock()
		return LoginResult{Status: "pending"}, nil
	}
	if t.Error != "" {
		s.CancelMicrosoft(id)
		return LoginResult{}, fmt.Errorf("微软登录失败 (%s): %s", t.Error, t.Description)
	}
	a, secret, e := s.minecraftToken(ctx, t)
	if e != nil {
		s.CancelMicrosoft(id)
		return LoginResult{}, e
	}
	a.ClientID = d.ClientID
	// Closing the login dialog cancels authorization before credentials are saved.
	s.mu.Lock()
	active := s.devices[id] == d
	s.mu.Unlock()
	if !active {
		return LoginResult{}, errors.New("登录已取消")
	}
	if e = s.saveAccount(a, secret); e != nil {
		return LoginResult{}, e
	}
	s.CancelMicrosoft(id)
	return LoginResult{Status: "complete", Account: &a}, nil
}
func (s *Service) minecraftToken(ctx context.Context, t oauthToken) (Account, tokenSecret, error) {
	var xbox struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			XUI []struct {
				UHS string `json:"uhs"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}
	if t.Access == "" {
		return Account{}, tokenSecret{}, errors.New("Microsoft 未返回 access token")
	}
	if e := s.jsonRequest(ctx, "POST", "https://user.auth.xboxlive.com/user/authenticate", map[string]any{"Properties": map[string]any{"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com", "RpsTicket": "d=" + t.Access}, "RelyingParty": "http://auth.xboxlive.com", "TokenType": "JWT"}, nil, &xbox); e != nil {
		return Account{}, tokenSecret{}, fmt.Errorf("Xbox 登录失败: %w", e)
	}
	if xbox.Token == "" || len(xbox.DisplayClaims.XUI) == 0 {
		return Account{}, tokenSecret{}, errors.New("Xbox 返回了无效的身份信息")
	}
	var xsts struct {
		Token string `json:"Token"`
	}
	if e := s.jsonRequest(ctx, "POST", "https://xsts.auth.xboxlive.com/xsts/authorize", map[string]any{"Properties": map[string]any{"SandboxId": "RETAIL", "UserTokens": []string{xbox.Token}}, "RelyingParty": "rp://api.minecraftservices.com/", "TokenType": "JWT"}, nil, &xsts); e != nil {
		return Account{}, tokenSecret{}, fmt.Errorf("Xbox 授权失败，请确认已创建 Xbox 档案且满足家庭账号授权要求: %w", e)
	}
	var mc struct {
		Access  string `json:"access_token"`
		Expires int    `json:"expires_in"`
	}
	if e := s.jsonRequest(ctx, "POST", "https://api.minecraftservices.com/authentication/login_with_xbox", map[string]string{"identityToken": "XBL3.0 x=" + xbox.DisplayClaims.XUI[0].UHS + ";" + xsts.Token}, nil, &mc); e != nil {
		return Account{}, tokenSecret{}, e
	}
	if mc.Access == "" {
		return Account{}, tokenSecret{}, errors.New("未取得 Minecraft 访问令牌")
	}
	var profile struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if e := s.jsonRequest(ctx, "GET", "https://api.minecraftservices.com/minecraft/profile", nil, map[string]string{"Authorization": "Bearer " + mc.Access}, &profile); e != nil {
		return Account{}, tokenSecret{}, fmt.Errorf("无法获取 Minecraft Java 档案，请确认账号已拥有游戏且应用获准访问 Minecraft API: %w", e)
	}
	if profile.ID == "" || profile.Name == "" {
		return Account{}, tokenSecret{}, errors.New("账号没有有效的 Minecraft Java 档案")
	}
	a := Account{ID: "ms-" + profile.ID, Name: profile.Name, UUID: profile.ID, Kind: "microsoft"}
	return a, tokenSecret{Access: mc.Access, Refresh: t.Refresh, Expires: time.Now().Add(time.Duration(mc.Expires) * time.Second)}, nil
}
func (s *Service) saveAccount(a Account, t tokenSecret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.putSecret(a.ID, t); e != nil {
		return e
	}
	found := false
	for i, v := range s.cfg.Accounts {
		if v.ID == a.ID {
			s.cfg.Accounts[i] = a
			found = true
			break
		}
	}
	if !found {
		s.cfg.Accounts = append(s.cfg.Accounts, a)
	}
	s.cfg.ActiveAccount = a.ID
	return s.saveLocked()
}

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
func validUUID(s string) bool {
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return false
	}
	for _, v := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", v) {
			return false
		}
	}
	return true
}
func offlineUUID(name string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + name))
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return hex.EncodeToString(sum[:])
}
func (s *Service) AddOffline(name string) (Account, error) {
	name = strings.TrimSpace(name)
	if !regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`).MatchString(name) {
		return Account{}, errors.New("玩家名需要 3–16 位英文字母、数字或下划线")
	}
	uuid := offlineUUID(name)
	a := Account{ID: "offline-" + uuid, Name: name, UUID: uuid, Kind: "offline"}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.cfg.Accounts {
		if v.ID == a.ID {
			s.cfg.ActiveAccount = a.ID
			return v, s.saveLocked()
		}
	}
	s.cfg.Accounts = append(s.cfg.Accounts, a)
	s.cfg.ActiveAccount = a.ID
	return a, s.saveLocked()
}
func (s *Service) accountToken(ctx context.Context, a Account) (tokenSecret, error) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if a.Kind == "offline" {
		return tokenSecret{Access: "0"}, nil
	}
	var t tokenSecret
	if e := s.getSecret(a.ID, &t); e != nil {
		return t, e
	}
	if a.Kind == "microsoft" {
		if time.Until(t.Expires) > 5*time.Minute {
			return t, nil
		}
		var next oauthToken
		if e := s.oauthForm(ctx, "/token", url.Values{"client_id": {a.ClientID}, "grant_type": {"refresh_token"}, "refresh_token": {t.Refresh}, "scope": {microsoftScope}}, &next); e != nil {
			return t, e
		}
		if next.Error != "" {
			return t, errors.New("微软登录已过期，请重新登录")
		}
		if next.Refresh == "" {
			next.Refresh = t.Refresh
		}
		_, secret, e := s.minecraftToken(ctx, next)
		if e != nil {
			return t, e
		}
		if e = s.putSecret(a.ID, secret); e != nil {
			return t, e
		}
		return secret, nil
	}
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
func (s *Service) RemoveAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.running) > 0 {
		return errors.New("请先退出运行中的游戏")
	}
	for i, a := range s.cfg.Accounts {
		if a.ID == id {
			s.cfg.Accounts = append(s.cfg.Accounts[:i], s.cfg.Accounts[i+1:]...)
			if s.cfg.ActiveAccount == id {
				s.cfg.ActiveAccount = ""
				if len(s.cfg.Accounts) > 0 {
					s.cfg.ActiveAccount = s.cfg.Accounts[0].ID
				}
			}
			if e := s.saveLocked(); e != nil {
				return e
			}
			e := os.Remove(s.secretPath(id))
			if os.IsNotExist(e) {
				return nil
			}
			return e
		}
	}
	return errors.New("账号不存在")
}
