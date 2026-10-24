package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const microsoftBase = "https://login.microsoftonline.com/consumers/oauth2/v2.0"
const microsoftScope = "XboxLive.signin offline_access"

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
	req.Header.Set("User-Agent", "YoloMCL/"+appVersion)
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
	id := s.publisher.MicrosoftClientID
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
		return Account{}, tokenSecret{}, errors.New("未收到 Microsoft 的 access token")
	}
	if e := s.jsonRequest(ctx, "POST", "https://user.auth.xboxlive.com/user/authenticate", map[string]any{"Properties": map[string]any{"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com", "RpsTicket": "d=" + t.Access}, "RelyingParty": "http://auth.xboxlive.com", "TokenType": "JWT"}, nil, &xbox); e != nil {
		return Account{}, tokenSecret{}, fmt.Errorf("登录 Xbox 失败: %w", e)
	}
	if xbox.Token == "" || len(xbox.DisplayClaims.XUI) == 0 {
		return Account{}, tokenSecret{}, errors.New("收到无效的 Xbox 身份信息")
	}
	var xsts struct {
		Token string `json:"Token"`
	}
	if e := s.jsonRequest(ctx, "POST", "https://xsts.auth.xboxlive.com/xsts/authorize", map[string]any{"Properties": map[string]any{"SandboxId": "RETAIL", "UserTokens": []string{xbox.Token}}, "RelyingParty": "rp://api.minecraftservices.com/", "TokenType": "JWT"}, nil, &xsts); e != nil {
		return Account{}, tokenSecret{}, fmt.Errorf("无法取得 Xbox 授权，请确认已创建 Xbox 档案且满足家庭账号授权要求: %w", e)
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

func (s *Service) refreshMicrosoft(ctx context.Context, a Account, t tokenSecret) (tokenSecret, error) {
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
