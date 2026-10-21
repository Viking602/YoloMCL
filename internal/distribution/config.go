// Package distribution holds publisher-owned service configuration. It is never
// part of player preferences or returned to the UI.
package distribution

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// encoded is injected by the publisher's build. It never enters player settings
// or public service snapshots.
var encoded string

type Config struct {
	MicrosoftClientID  string `json:"microsoftClientId"`
	CurseForgeProxyURL string `json:"curseForgeProxyUrl"`
	// The publisher can inject a direct key, as XMCL does, or keep it on a relay.
	// json:"-" ensures credentials are never included in public serialization.
	CurseForgeAPIKey string `json:"-"`
}

func Load() (Config, error) {
	c := Config{}
	if encoded != "" {
		b, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil {
			return c, errors.New("发行服务配置编码无效")
		}
		if e = json.Unmarshal(b, &c); e != nil {
			return c, errors.New("发行服务配置无效")
		}
		var private struct {
			CurseForgeAPIKey string `json:"curseForgeApiKey"`
		}
		if e = json.Unmarshal(b, &private); e != nil {
			return c, errors.New("发行服务配置无效")
		}
		c.CurseForgeAPIKey = private.CurseForgeAPIKey
	}
	// Precedence: CURSEFORGE_API_KEY env/.env, then ldflags-obfuscated key,
	// then any key already loaded from the publisher encoded blob.
	if c.CurseForgeProxyURL == "" {
		if v := overrideCurseForgeKey(); v != "" {
			c.CurseForgeAPIKey = v
		} else if v := embeddedCurseForgeKey(); v != "" {
			c.CurseForgeAPIKey = v
		}
	}
	if os.Getenv("MCL_DEVELOPMENT") == "1" {
		if v := os.Getenv("MCL_MICROSOFT_CLIENT_ID"); v != "" {
			c.MicrosoftClientID = v
		}
		if v := os.Getenv("MCL_CURSEFORGE_PROXY_URL"); v != "" {
			c.CurseForgeProxyURL = v
		}
		if v := os.Getenv("MCL_CURSEFORGE_API_KEY"); v != "" {
			c.CurseForgeAPIKey = v
		}
	}
	return c, c.Validate()
}
func (c *Config) Validate() error {
	c.MicrosoftClientID = strings.TrimSpace(c.MicrosoftClientID)
	if c.MicrosoftClientID != "" && !regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(c.MicrosoftClientID) {
		return errors.New("发行版 Microsoft 应用标识无效")
	}
	c.CurseForgeProxyURL = strings.TrimRight(strings.TrimSpace(c.CurseForgeProxyURL), "/")
	if c.CurseForgeProxyURL != "" {
		u, e := url.Parse(c.CurseForgeProxyURL)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("发行版资源服务必须使用有效的 HTTPS 地址")
		}
	}
	return nil
}
func (c Config) HasMicrosoft() bool  { return c.MicrosoftClientID != "" }
func (c Config) HasCurseForge() bool { return c.CurseForgeProxyURL != "" || c.CurseForgeAPIKey != "" }
