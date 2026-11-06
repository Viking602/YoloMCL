package launcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func accountSkinFixture(t *testing.T, kind string) (*Service, Account, []byte, []byte, string) {
	t.Helper()
	s := testService(t)
	account := Account{ID: "avatar-fixture", Kind: kind, UUID: "00000000-0000-0000-0000-000000000001", Server: "https://skin.example/yggdrasil"}
	s.cfg.Accounts = []Account{account}
	if err := atomicJSON(filepath.Join(s.dir, "config.json"), s.cfg); err != nil {
		t.Fatal(err)
	}
	var skin bytes.Buffer
	if err := png.Encode(&skin, image.NewNRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	textures := jsonBytes(t, map[string]any{"textures": map[string]any{
		"SKIN": map[string]any{"url": "http://textures.minecraft.net/texture/avatar", "metadata": map[string]string{"model": "slim"}},
		"CAPE": map[string]string{"url": "https://broken.example/cape.png"},
	}})
	profile := jsonBytes(t, map[string]any{"properties": []map[string]string{{"name": "textures", "value": base64.StdEncoding.EncodeToString(textures)}}})
	endpoint := account.Server + "/sessionserver/session/minecraft/profile/00000000000000000000000000000001?unsigned=false"
	if kind == "microsoft" {
		endpoint = "https://sessionserver.mojang.com/session/minecraft/profile/00000000000000000000000000000001?unsigned=false"
	}
	return s, account, skin.Bytes(), profile, endpoint
}

func TestAccountSkinLoadsWithoutCredentialsOrCape(t *testing.T) {
	for _, kind := range []string{"microsoft", "yggdrasil"} {
		t.Run(kind, func(t *testing.T) {
			s, account, skin, profile, endpoint := accountSkinFixture(t, kind)
			var calls []string
			s.http = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				calls = append(calls, req.URL.String())
				if req.Header.Get("Authorization") != "" {
					t.Error("public avatar sent an access token")
				}
				var body []byte
				switch req.URL.String() {
				case endpoint:
					body = profile
				case "https://textures.minecraft.net/texture/avatar":
					body = skin
				default:
					t.Errorf("unexpected avatar request: %s", req.URL)
					return nil, errors.New("unexpected request")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header), Request: req}, nil
			})}
			got, err := s.LoadAccountSkin(context.Background(), account.ID)
			if err != nil || got.Model != "slim" || got.Skin != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(skin) || len(calls) != 2 {
				t.Fatalf("skin request failed or fetched unrelated data: %+v %v %v", got, err, calls)
			}
			if _, err := os.Stat(s.secretPath(account.ID)); !os.IsNotExist(err) {
				t.Fatal("avatar touched credential storage", err)
			}
			cached, err := s.CachedAccountSkin(context.Background(), account.ID)
			if err != nil || cached != got || len(calls) != 2 {
				t.Fatal("cache performed network I/O or lost skin", err)
			}
			reopened, err := New(s.dir)
			if err != nil {
				t.Fatal(err)
			}
			cached, err = reopened.CachedAccountSkin(context.Background(), account.ID)
			if err != nil || cached != got {
				t.Fatal("avatar cache did not survive restart", err)
			}
			// Changing a server must not reuse another provider's cached skin.
			reopened.cfg.Accounts[0].Server = "https://another.example/yggdrasil"
			if _, err = reopened.CachedAccountSkin(context.Background(), account.ID); !os.IsNotExist(err) {
				t.Fatal("provider cache collision", err)
			}
		})
	}
}

func TestAccountSkinFailurePreservesCacheAndRecovers(t *testing.T) {
	s, account, skin, profile, endpoint := accountSkinFixture(t, "microsoft")
	fakeHTTP(t, s, map[string][]byte{endpoint: profile, "https://textures.minecraft.net/texture/avatar": skin})
	want, err := s.LoadAccountSkin(context.Background(), account.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.http = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("unavailable")), Request: req}, nil
	})}
	if _, err := s.LoadAccountSkin(context.Background(), account.ID); err == nil {
		t.Fatal("network failure hidden as success")
	}
	if got, err := s.CachedAccountSkin(context.Background(), account.ID); err != nil || got != want {
		t.Fatal("network failure erased a valid avatar", err)
	}
	writeTestFile(t, s.accountSkinCachePath(account), []byte("broken cache"))
	if _, err := s.CachedAccountSkin(context.Background(), account.ID); err == nil {
		t.Fatal("invalid cache accepted")
	}
	fakeHTTP(t, s, map[string][]byte{endpoint: profile, "https://textures.minecraft.net/texture/avatar": skin})
	if _, err := s.LoadAccountSkin(context.Background(), account.ID); err != nil {
		t.Fatal("bad cache blocked recovery", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.LoadAccountSkin(ctx, account.ID); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAccountSkinDefaultAndReadOnlyCache(t *testing.T) {
	s, account, skin, profile, endpoint := accountSkinFixture(t, "microsoft")
	writeTestFile(t, filepath.Join(s.dir, "cache", "account-skins"), []byte("cache directory blocked"))
	fakeHTTP(t, s, map[string][]byte{endpoint: profile, "https://textures.minecraft.net/texture/avatar": skin})
	if got, err := s.LoadAccountSkin(context.Background(), account.ID); err != nil || got.Skin == "" {
		t.Fatal("cache write failure hid downloaded skin", err)
	}
	fakeHTTP(t, s, map[string][]byte{endpoint: []byte(`{"properties":[]}`)})
	if got, err := s.LoadAccountSkin(context.Background(), account.ID); err != nil || got.Skin != "" || got.Model != "default" {
		t.Fatal("no custom texture is a valid default skin", got, err)
	}
}

func TestAccountSkinRejectsInvalidAndOversizedTextures(t *testing.T) {
	s, account, _, profile, endpoint := accountSkinFixture(t, "microsoft")
	for _, size := range []image.Point{{32, 32}, {64, 16}, {1025, 1025}, {64, 64}} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))); err != nil {
			t.Fatal(err)
		}
		data := buf.Bytes()
		if size.X == 64 && size.Y == 64 {
			data = data[:40]
		}
		fakeHTTP(t, s, map[string][]byte{endpoint: profile, "https://textures.minecraft.net/texture/avatar": data})
		if _, err := s.LoadAccountSkin(context.Background(), account.ID); err == nil {
			t.Fatal("invalid PNG accepted", size)
		}
		if _, err := os.Stat(s.accountSkinCachePath(account)); !os.IsNotExist(err) {
			t.Fatal("invalid texture persisted", err)
		}
	}
}
