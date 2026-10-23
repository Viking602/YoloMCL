// Package buildconfig prepares the same publisher configuration and compiler
// arguments on every host. It is used by build tooling, not by the application.
package buildconfig

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Viking602/YoloMCL/internal/distribution"
	"github.com/Viking602/YoloMCL/internal/dotenv"
)

// Load reads only the repository's .env and the explicitly selected publisher
// file. Precedence is publisher file, environment, then repository .env.
func Load(root, publisherPath string, getenv func(string) string) (distribution.Config, error) {
	var config distribution.Config
	if publisherPath != "" {
		data, err := os.ReadFile(publisherPath)
		if err != nil {
			return config, fmt.Errorf("read publisher configuration: %w", err)
		}
		var input struct {
			MicrosoftClientID  string `json:"microsoftClientId"`
			CurseForgeProxyURL string `json:"curseForgeProxyUrl"`
			CurseForgeAPIKey   string `json:"curseForgeApiKey"`
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return config, errors.New("invalid publisher configuration JSON")
		}
		config = distribution.Config{
			MicrosoftClientID:  input.MicrosoftClientID,
			CurseForgeProxyURL: input.CurseForgeProxyURL,
			CurseForgeAPIKey:   input.CurseForgeAPIKey,
		}
	}
	values := map[string]string{}
	file, err := os.Open(filepath.Join(root, ".env"))
	if err == nil {
		defer file.Close()
		values, err = dotenv.Parse(file, "MS_CLIENT_ID", "MCL_MICROSOFT_CLIENT_ID", "CURSEFORGE_API_KEY", "MCL_CURSEFORGE_API_KEY")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config, fmt.Errorf("read repository .env: %w", err)
	}
	lookup := func(keys ...string) string {
		for _, key := range keys {
			if value := strings.TrimSpace(getenv(key)); value != "" {
				return value
			}
		}
		for _, key := range keys {
			if value := strings.TrimSpace(values[key]); value != "" {
				return value
			}
		}
		return ""
	}
	if strings.TrimSpace(config.MicrosoftClientID) == "" {
		config.MicrosoftClientID = lookup("MS_CLIENT_ID", "MCL_MICROSOFT_CLIENT_ID")
	}
	if strings.TrimSpace(config.CurseForgeAPIKey) == "" {
		config.CurseForgeAPIKey = lookup("CURSEFORGE_API_KEY", "MCL_CURSEFORGE_API_KEY")
	}
	config.CurseForgeAPIKey = strings.TrimSpace(config.CurseForgeAPIKey)
	err = config.Validate()
	return config, err
}

// Arguments returns go build arguments. Callers must not log the arguments,
// since linker values contain recoverable publisher configuration.
func Arguments(config distribution.Config, goos, goflags, output string, release bool) ([]string, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if release && (!config.HasMicrosoft() || !config.HasCurseForge()) {
		return nil, errors.New("release requires a Microsoft client ID and a CurseForge API key or HTTPS catalog relay")
	}
	data, err := json.Marshal(config) // Config intentionally omits the private key.
	if err != nil {
		return nil, fmt.Errorf("encode publisher configuration: %w", err)
	}
	pkg := reflect.TypeFor[distribution.Config]().PkgPath()
	flags := "-s -w -X " + pkg + ".encoded=" + base64.StdEncoding.EncodeToString(data) +
		" -X " + pkg + ".cfKeyObf=" + distribution.EncodeCurseForgeKey(config.CurseForgeAPIKey)
	if goos == "windows" {
		flags += " -H=windowsgui"
	}
	args := []string{"build", "-trimpath"}
	if release {
		tags := []string{"mygo_noinspector"}
		for _, field := range strings.Fields(goflags) {
			name, value, ok := strings.Cut(strings.Trim(field, "\"'"), "=")
			if ok && (name == "-tags" || name == "--tags") {
				tags = append(tags, strings.Trim(value, "\"'"))
			}
		}
		args = append(args, "-tags="+strings.Join(tags, ","))
		flags += " -X github.com/egoist/mygo.production=1"
	}
	return append(args, "-ldflags", flags, "-o", output, "."), nil
}
