package launcher

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"path/filepath"
)

// Original Minecraft 26.1.2 assets, embedded unchanged so first-run and offline
// previews have the real Steve/Alex textures. See THIRD_PARTY_NOTICES.md.
//
//go:embed assets/skins/steve.png
var vanillaStevePNG []byte

//go:embed assets/skins/alex.png
var vanillaAlexPNG []byte

// VanillaSkinPNG returns a caller-owned copy of the built-in vanilla texture.
// There is no filesystem scan, cache dependency or network request.
func VanillaSkinPNG(model string) []byte {
	if model == "slim" {
		return bytes.Clone(vanillaAlexPNG)
	}
	return bytes.Clone(vanillaStevePNG)
}

func (s *Service) VanillaSkin(ctx context.Context, model string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return VanillaSkinPNG(model), nil
}

// SetOfflineModel remembers an offline account's Steve/Alex pick.
func (s *Service) SetOfflineModel(id, model string) error {
	a, e := s.getAccount(id)
	if e != nil {
		return e
	}
	if a.Kind != "offline" {
		return errors.New("只有离线账号可以这样设置")
	}
	if model != "slim" {
		model = "default"
	}
	return atomicJSON(filepath.Join(s.dir, "cosmetics", a.ID, "model.json"), model)
}
