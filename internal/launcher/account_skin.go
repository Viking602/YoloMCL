package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AccountSkin contains only the texture needed to display an account avatar.
// It never contains credentials or the player's cape inventory.
type AccountSkin struct {
	Skin  string `json:"skin"`
	Model string `json:"model"`
}

// LoadAccountSkin reads public profile textures independently of authenticated
// wardrobe operations. An expired login or broken cape must not hide a head.
func (s *Service) LoadAccountSkin(ctx context.Context, id string) (AccountSkin, error) {
	if err := ctx.Err(); err != nil {
		return AccountSkin{}, err
	}
	account, err := s.getAccount(id)
	if err != nil {
		return AccountSkin{}, err
	}
	if account.Kind == "offline" {
		return s.offlineAccountSkin(ctx, account)
	}
	out := AccountSkin{Model: "default"}
	if !validUUID(account.UUID) {
		return out, errors.New("invalid account UUID")
	}
	base := strings.TrimRight(account.Server, "/") + "/sessionserver"
	switch account.Kind {
	case "microsoft":
		base = "https://sessionserver.mojang.com"
	case "yggdrasil":
	default:
		return out, errors.New("unsupported account skin provider")
	}
	var profile struct {
		Properties []struct{ Name, Value string }
	}
	endpoint := base + "/session/minecraft/profile/" + strings.ReplaceAll(account.UUID, "-", "") + "?unsigned=false"
	if err := s.getJSON(ctx, endpoint, &profile); err != nil {
		return out, err
	}
	for _, property := range profile.Properties {
		if property.Name != "textures" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(property.Value)
		if err != nil {
			return out, err
		}
		var payload struct {
			Textures struct {
				Skin struct {
					URL      string
					Metadata struct{ Model string }
				}
			}
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return out, err
		}
		out.Skin, err = s.imageData(ctx, payload.Textures.Skin.URL)
		if err != nil {
			return out, err
		}
		if payload.Textures.Skin.Metadata.Model == "slim" {
			out.Model = "slim"
		}
		break
	}
	if err := validateAccountSkin(out); err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	// A full or read-only cache must not prevent showing a downloaded skin.
	_ = atomicJSON(s.accountSkinCachePath(account), struct {
		Schema int `json:"schema"`
		AccountSkin
	}{1, out})
	return out, nil
}

func (s *Service) accountSkinCachePath(account Account) string {
	identity := strings.Join([]string{account.ID, account.Kind, account.UUID, account.Server}, "\x00")
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(s.dir, "cache", "account-skins", hex.EncodeToString(sum[:])+".json")
}

// CachedAccountSkin returns the last valid public skin without network access.
// Callers can display it immediately while refreshing in the background.
func (s *Service) CachedAccountSkin(ctx context.Context, id string) (AccountSkin, error) {
	account, err := s.getAccount(id)
	if err != nil {
		return AccountSkin{}, err
	}
	if account.Kind == "offline" {
		return s.offlineAccountSkin(ctx, account)
	}
	data, err := readAccountSkinFile(ctx, s.accountSkinCachePath(account), 6<<20)
	if err != nil {
		return AccountSkin{}, err
	}
	var cached struct {
		Schema int `json:"schema"`
		AccountSkin
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return AccountSkin{}, err
	}
	if cached.Schema != 1 {
		return AccountSkin{}, errors.New("unsupported account skin cache")
	}
	if err := validateAccountSkin(cached.AccountSkin); err != nil {
		return AccountSkin{}, err
	}
	return cached.AccountSkin, nil
}

func (s *Service) offlineAccountSkin(ctx context.Context, account Account) (AccountSkin, error) {
	out := AccountSkin{Model: "default"}
	data, err := readAccountSkinFile(ctx, filepath.Join(s.dir, "cosmetics", account.ID, "skin.png"), 4<<20)
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}
	if len(data) > 0 {
		out.Skin = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	}
	if data, err := readAccountSkinFile(ctx, filepath.Join(s.dir, "cosmetics", account.ID, "model.json"), 128); err == nil {
		var model string
		if json.Unmarshal(data, &model) == nil && model == "slim" {
			out.Model = model
		}
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	return out, validateAccountSkin(out)
}

func readAccountSkinFile(ctx context.Context, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(modReader{ctx: ctx, r: f}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("account skin data is too large")
	}
	return data, nil
}

func validateAccountSkin(skin AccountSkin) error {
	if skin.Model != "default" && skin.Model != "slim" {
		return errors.New("invalid account skin model")
	}
	if skin.Skin == "" {
		return nil
	}
	encoded, ok := strings.CutPrefix(skin.Skin, "data:image/png;base64,")
	if !ok || len(encoded) > base64.StdEncoding.EncodedLen(4<<20) {
		return errors.New("invalid account skin data")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if config.Width < 64 || config.Width > 1024 || config.Width%64 != 0 || config.Height != config.Width && config.Height != config.Width/2 {
		return errors.New("invalid account skin dimensions")
	}
	_, err = png.Decode(bytes.NewReader(data))
	return err
}
