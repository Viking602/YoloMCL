package launcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

type Cape struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
}
type Appearance struct {
	Skin    string `json:"skin"`
	Cape    string `json:"cape"`
	Model   string `json:"model"`
	Capes   []Cape `json:"capes"`
	CanSkin bool   `json:"canSkin"`
	CanCape bool   `json:"canCape"`
}

// Profiles can still contain legacy HTTP texture URLs. Normalize both the
// active textures and the selectable capes before they reach HTTPS-only loaders.
func textureURL(raw string) string {
	if rest, ok := strings.CutPrefix(raw, "http://"); ok {
		return "https://" + rest
	}
	return raw
}

func (s *Service) imageData(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	r, e := s.request(ctx, "GET", textureURL(raw), nil, nil)
	if e != nil {
		return "", e
	}
	defer r.Body.Close()
	b, e := io.ReadAll(io.LimitReader(r.Body, (4<<20)+1))
	if e != nil {
		return "", e
	}
	if len(b) > 4<<20 {
		return "", errors.New("材质图片超过 4 MiB")
	}
	if _, e = png.DecodeConfig(bytes.NewReader(b)); e != nil {
		return "", errors.New("材质不是有效 PNG 图片")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
}
func (s *Service) Appearance(ctx context.Context, id string) (Appearance, error) {
	a, e := s.getAccount(id)
	if e != nil {
		return Appearance{}, e
	}
	out := Appearance{Model: "default", Capes: []Cape{}}
	if a.Kind == "offline" {
		out.CanSkin = true
		out.CanCape = true
		for _, kind := range []string{"skin", "cape"} {
			b, e := os.ReadFile(filepath.Join(s.dir, "cosmetics", a.ID, kind+".png"))
			if e == nil {
				data := "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
				if kind == "skin" {
					out.Skin = data
				} else {
					out.Cape = data
				}
			}
		}
		var model string
		b, e := os.ReadFile(filepath.Join(s.dir, "cosmetics", a.ID, "model.json"))
		if e == nil && json.Unmarshal(b, &model) == nil {
			out.Model = model
		}
		return out, nil
	}
	if a.Kind == "microsoft" {
		t, e := s.accountToken(ctx, a)
		if e != nil {
			return out, e
		}
		var profile struct {
			Skins []struct {
				URL     string `json:"url"`
				Variant string `json:"variant"`
				State   string `json:"state"`
			} `json:"skins"`
			Capes []struct {
				ID    string `json:"id"`
				URL   string `json:"url"`
				Alias string `json:"alias"`
				State string `json:"state"`
			} `json:"capes"`
		}
		if e = s.jsonRequest(ctx, "GET", "https://api.minecraftservices.com/minecraft/profile", nil, map[string]string{"Authorization": "Bearer " + t.Access}, &profile); e != nil {
			return out, e
		}
		out.CanSkin = true
		out.CanCape = true
		for _, v := range profile.Skins {
			if v.State == "ACTIVE" {
				out.Skin = v.URL
				if strings.EqualFold(v.Variant, "SLIM") {
					out.Model = "slim"
				}
			}
		}
		for _, v := range profile.Capes {
			out.Capes = append(out.Capes, Cape{ID: v.ID, Name: v.Alias, URL: textureURL(v.URL), Active: v.State == "ACTIVE"})
			if v.State == "ACTIVE" {
				out.Cape = v.URL
			}
		}
	} else {
		var profile struct {
			Properties []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"properties"`
		}
		if e = s.getJSON(ctx, a.Server+"/sessionserver/session/minecraft/profile/"+a.UUID+"?unsigned=false", &profile); e != nil {
			return out, e
		}
		for _, p := range profile.Properties {
			if p.Name == "uploadableTextures" {
				values := strings.Split(p.Value, ",")
				out.CanSkin = contains(values, "skin")
				out.CanCape = contains(values, "cape")
			}
			if p.Name == "textures" {
				b, e := base64.StdEncoding.DecodeString(p.Value)
				if e != nil {
					return out, e
				}
				var v struct {
					Textures map[string]struct {
						URL      string `json:"url"`
						Metadata struct {
							Model string `json:"model"`
						} `json:"metadata"`
					} `json:"textures"`
				}
				if e = json.Unmarshal(b, &v); e != nil {
					return out, e
				}
				out.Skin = v.Textures["SKIN"].URL
				out.Cape = v.Textures["CAPE"].URL
				if v.Textures["SKIN"].Metadata.Model == "slim" {
					out.Model = "slim"
				}
			}
		}
	}
	if out.Skin, e = s.imageData(ctx, out.Skin); e != nil {
		return out, fmt.Errorf("读取皮肤失败: %w", e)
	}
	if out.Cape, e = s.imageData(ctx, out.Cape); e != nil {
		return out, fmt.Errorf("读取披风失败: %w", e)
	}
	return out, nil
}
func textureBytes(data, kind string) ([]byte, error) {
	data = strings.TrimPrefix(data, "data:image/png;base64,")
	if len(data) > 6<<20 {
		return nil, errors.New("图片超过 4 MiB")
	}
	b, e := base64.StdEncoding.DecodeString(data)
	if e != nil {
		return nil, e
	}
	c, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil {
		return nil, errors.New("请上传有效的 PNG 图片")
	}
	if c.Width > 1024 || c.Height > 1024 || c.Width < 32 || c.Height < 16 {
		return nil, errors.New("材质尺寸无效（最大 1024×1024）")
	}
	if kind == "skin" && c.Width != c.Height && c.Width != c.Height*2 {
		return nil, errors.New("皮肤宽高比应为 1:1 或 2:1")
	}
	if kind == "cape" && c.Width != c.Height*2 {
		return nil, errors.New("披风宽高比应为 2:1")
	}
	return b, nil
}
func (s *Service) UploadTexture(ctx context.Context, id, kind, model, data string) error {
	if kind != "skin" && kind != "cape" {
		return errors.New("未知材质类型")
	}
	if model != "slim" {
		model = "default"
	}
	a, e := s.getAccount(id)
	if e != nil {
		return e
	}
	b, e := textureBytes(data, kind)
	if e != nil {
		return e
	}
	if a.Kind == "offline" {
		if e = atomicBytes(filepath.Join(s.dir, "cosmetics", a.ID, kind+".png"), b, 0600); e != nil {
			return e
		}
		if kind == "skin" {
			return atomicJSON(filepath.Join(s.dir, "cosmetics", a.ID, "model.json"), model)
		}
		return nil
	}
	if a.Kind == "microsoft" {
		if kind != "skin" {
			return errors.New("官方账号只能使用已拥有的披风")
		}
		c, _ := png.DecodeConfig(bytes.NewReader(b))
		if c.Width != 64 || (c.Height != 64 && c.Height != 32) {
			return errors.New("官方皮肤尺寸必须是 64×64 或 64×32")
		}
	}
	if a.Kind == "yggdrasil" {
		appearance, e := s.Appearance(ctx, id)
		if e != nil {
			return e
		}
		if kind == "skin" && !appearance.CanSkin || kind == "cape" && !appearance.CanCape {
			return errors.New("认证站未授权上传此类材质")
		}
	}
	t, e := s.accountToken(ctx, a)
	if e != nil {
		return e
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if a.Kind == "microsoft" {
		v := "classic"
		if model == "slim" {
			v = "slim"
		}
		if e = writer.WriteField("variant", v); e != nil {
			return e
		}
	} else {
		v := ""
		if model == "slim" {
			v = "slim"
		}
		if e = writer.WriteField("model", v); e != nil {
			return e
		}
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="texture.png"`)
	h.Set("Content-Type", "image/png")
	part, e := writer.CreatePart(h)
	if e != nil {
		return e
	}
	if _, e = part.Write(b); e != nil {
		return e
	}
	if e = writer.Close(); e != nil {
		return e
	}
	method, endpoint := "POST", "https://api.minecraftservices.com/minecraft/profile/skins"
	if a.Kind == "yggdrasil" {
		method = "PUT"
		endpoint = a.Server + "/api/user/profile/" + a.UUID + "/" + kind
	}
	r, e := s.request(ctx, method, endpoint, &buf, map[string]string{"Authorization": "Bearer " + t.Access, "Content-Type": writer.FormDataContentType()})
	if e != nil {
		return e
	}
	r.Body.Close()
	return nil
}
func (s *Service) SetCape(ctx context.Context, id, capeID string) error {
	a, e := s.getAccount(id)
	if e != nil {
		return e
	}
	if a.Kind != "microsoft" {
		return errors.New("第三方账号请使用披风上传或重置")
	}
	t, e := s.accountToken(ctx, a)
	if e != nil {
		return e
	}
	if capeID == "" {
		return s.jsonRequest(ctx, "DELETE", "https://api.minecraftservices.com/minecraft/profile/capes/active", nil, map[string]string{"Authorization": "Bearer " + t.Access}, nil)
	}
	appearance, e := s.Appearance(ctx, id)
	if e != nil {
		return e
	}
	found := false
	for _, c := range appearance.Capes {
		if c.ID == capeID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("账号未拥有此披风")
	}
	return s.jsonRequest(ctx, "PUT", "https://api.minecraftservices.com/minecraft/profile/capes/active", map[string]string{"capeId": capeID}, map[string]string{"Authorization": "Bearer " + t.Access}, nil)
}
func (s *Service) ResetTexture(ctx context.Context, id, kind string) error {
	if a, e := s.getAccount(id); e == nil && a.Kind == "offline" {
		if kind != "skin" && kind != "cape" {
			return errors.New("未知材质类型")
		}
		e = os.Remove(filepath.Join(s.dir, "cosmetics", a.ID, kind+".png"))
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	if kind != "skin" && kind != "cape" {
		return errors.New("未知材质类型")
	}
	a, e := s.getAccount(id)
	if e != nil {
		return e
	}
	t, e := s.accountToken(ctx, a)
	if e != nil {
		return e
	}
	endpoint := "https://api.minecraftservices.com/minecraft/profile/skins/active"
	if kind == "cape" {
		endpoint = "https://api.minecraftservices.com/minecraft/profile/capes/active"
	}
	if a.Kind == "yggdrasil" {
		appearance, e := s.Appearance(ctx, id)
		if e != nil {
			return e
		}
		if kind == "skin" && !appearance.CanSkin || kind == "cape" && !appearance.CanCape {
			return errors.New("认证站未授权修改此类材质")
		}
		endpoint = a.Server + "/api/user/profile/" + a.UUID + "/" + kind
	}
	return s.jsonRequest(ctx, "DELETE", endpoint, nil, map[string]string{"Authorization": "Bearer " + t.Access}, nil)
}
