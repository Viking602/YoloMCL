// Package config holds publisher-owned service configuration. It is never
// part of player preferences or returned to the UI.
package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
)

// msClientID is the Azure application (client) ID on Mojang's allow list,
// accepted as a legacy linker override. The shell build script now places
// MS_CLIENT_ID in encoded on every platform. Builds without either keep
// Microsoft sign-in off.
var msClientID string

// encoded is injected by the publisher's build. It never enters player settings
// or public service snapshots.
var encoded string

type Config struct {
	MicrosoftClientID string `json:"microsoftClientId"`
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
	if v := overrideCurseForgeKey(); v != "" {
		c.CurseForgeAPIKey = v
		keySource = "env"
	} else if v := embeddedCurseForgeKey(); v != "" {
		c.CurseForgeAPIKey = v
		keySource = "embedded"
	} else if c.CurseForgeAPIKey != "" {
		keySource = "publisher"
	}
	if c.MicrosoftClientID == "" {
		c.MicrosoftClientID = strings.TrimSpace(msClientID)
	}
	if os.Getenv("MCL_DEVELOPMENT") == "1" {
		if v := os.Getenv("MCL_MICROSOFT_CLIENT_ID"); v != "" {
			c.MicrosoftClientID = v
		}
		if v := os.Getenv("MCL_CURSEFORGE_API_KEY"); v != "" {
			c.CurseForgeAPIKey = v
		}
	}
	err := c.Validate()
	return c, err
}
func (c *Config) Validate() error {
	c.MicrosoftClientID = strings.TrimSpace(c.MicrosoftClientID)
	if c.MicrosoftClientID != "" && !regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(c.MicrosoftClientID) {
		return errors.New("发行版 Microsoft 应用标识无效")
	}
	return nil
}
func (c Config) HasMicrosoft() bool  { return c.MicrosoftClientID != "" }
func (c Config) HasCurseForge() bool { return c.CurseForgeAPIKey != "" }

var keySource = "none"

// CurseForgeKeySource says where the CurseForge key came from (env,
// embedded, publisher or none). Never the value.
func CurseForgeKeySource() string { return keySource }
