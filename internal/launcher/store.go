package launcher

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	publisherconfig "github.com/Viking602/YoloMCL/internal/config"
)

func New(dataDir string) (*Service, error) {
	publisher, e := publisherconfig.Load()
	if e != nil {
		return nil, e
	}
	if dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(base, "YoloMCL")
		migrateDataRoot(filepath.Join(base, "MCL"), dataDir)
	}
	dataDir, e = filepath.Abs(dataDir)
	if e != nil {
		return nil, e
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(dataDir); err == nil {
		dataDir = resolved
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Some networks terminate HTTP/2 connections to Minecraft CDNs. HTTP/1.1
	// also streams downloads and retains normal certificate verification.
	transport.ForceAttemptHTTP2 = false
	transport.Protocols = new(http.Protocols)
	transport.Protocols.SetHTTP1(true)
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}}
	transport.MaxIdleConnsPerHost = 8
	transport.ResponseHeaderTimeout = 30 * time.Second
	s := &Service{dir: dataDir, http: &http.Client{Transport: withNetFault(transport), CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 8 {
			return errors.New("重定向次数过多")
		}
		if req.URL.Scheme != "https" {
			return errors.New("拒绝降级到非 HTTPS 地址")
		}
		if len(via) > 0 && via[0].Method != "GET" && via[0].Method != "HEAD" && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			return errors.New("拒绝将认证请求重定向到其他域名")
		}
		return nil
	}}, jobs: map[string]*jobState{}, running: map[string]*runningGame{}, rootLocks: map[string]bool{}, devices: map[string]*deviceSession{}}
	s.cfg = config{Schema: 1, Settings: Settings{Language: NormalizeLanguage(os.Getenv("MCL_LANG")), MemoryMB: 4096}, Roots: []Root{}, Accounts: []Account{}}
	b, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err == nil {
		if err := json.Unmarshal(b, &s.cfg); err != nil {
			return nil, fmt.Errorf("配置文件损坏，原文件已保留: %w", err)
		}
		if s.cfg.Schema != 1 {
			return nil, errors.New("不支持此配置版本")
		}
		var previous struct {
			Settings map[string]json.RawMessage `json:"settings"`
		}
		if json.Unmarshal(b, &previous) == nil {
			if _, present := previous.Settings["useRootSettings"]; !present {
				for _, r := range s.cfg.Roots {
					if r.JavaPath != "" || r.MemoryMB > 0 || r.JVMArgs != "" || len(r.VersionConfigs) > 0 {
						s.cfg.Settings.UseRootSettings = true
						break
					}
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	s.cfg.Settings.Language = NormalizeLanguage(s.cfg.Settings.Language)
	s.publisher = publisher
	return s, nil
}
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func atomicJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicBytes(path, b, 0600)
}
func atomicBytes(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mcl-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (s *Service) settings() Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.cfg.Settings }
func (s *Service) getRoot(id string) (Root, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.cfg.Roots {
		if r.ID == id {
			return r.clone(), nil
		}
	}
	return Root{}, errors.New("找不到 Minecraft 目录")
}
func (s *Service) getAccount(id string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.cfg.Accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return Account{}, errors.New("请先添加并选择账号")
}
func (s *Service) secretPath(key string) string {
	return filepath.Join(s.dir, "credentials", key+".bin")
}
func (s *Service) putSecret(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b, err = seal(b)
	if err != nil {
		return err
	}
	return atomicBytes(s.secretPath(key), b, 0600)
}
func (s *Service) getSecret(key string, v any) error {
	b, err := os.ReadFile(s.secretPath(key))
	if err != nil {
		return errors.New("登录凭据不可用，请重新登录")
	}
	b, err = unseal(b)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func (s *Service) SaveSettings(v Settings) error {
	v.Language = NormalizeLanguage(v.Language)
	v.JavaPath = strings.TrimSpace(v.JavaPath)
	v.JVMArgs = strings.TrimSpace(v.JVMArgs)
	if _, e := ParseJVMArgs(v.JVMArgs); e != nil {
		return e
	}
	if v.MemoryMode != "manual" {
		v.MemoryMode = "" // auto
	}
	if v.MemoryMB < 512 || v.MemoryMB > 131072 {
		return errors.New("内存应在 512–131072 MB 之间")
	}
	if info, e := SystemMemory(); e == nil && v.MemoryMB > info.MaxMB {
		return fmt.Errorf("设置内存超过系统内存上限 %d MB", info.MaxMB)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	draft.Settings = v
	return s.saveConfigLocked(draft)
}
