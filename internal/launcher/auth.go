package launcher

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"
)

type tokenSecret struct {
	Access      string    `json:"access"`
	Refresh     string    `json:"refresh"`
	ClientToken string    `json:"clientToken"`
	Expires     time.Time `json:"expires"`
}

func (s *Service) saveAccount(a Account, t tokenSecret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	if e := s.putSecret(a.ID, t); e != nil {
		return e
	}
	found := false
	for i, v := range draft.Accounts {
		if v.ID == a.ID {
			draft.Accounts[i] = a
			found = true
			break
		}
	}
	if !found {
		draft.Accounts = append(draft.Accounts, a)
	}
	draft.ActiveAccount = a.ID
	return s.saveConfigLocked(draft)
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
	draft := s.cfg.clone()
	for _, v := range draft.Accounts {
		if v.ID == a.ID {
			draft.ActiveAccount = a.ID
			return v, s.saveConfigLocked(draft)
		}
	}
	draft.Accounts = append(draft.Accounts, a)
	draft.ActiveAccount = a.ID
	return a, s.saveConfigLocked(draft)
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
		return s.refreshMicrosoft(ctx, a, t)
	}
	return s.refreshThird(ctx, a, t)
}
func (s *Service) RemoveAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	if len(s.running) > 0 {
		return errors.New("请先退出运行中的游戏")
	}
	for i, a := range draft.Accounts {
		if a.ID == id {
			draft.Accounts = append(draft.Accounts[:i], draft.Accounts[i+1:]...)
			if draft.ActiveAccount == id {
				draft.ActiveAccount = ""
				if len(draft.Accounts) > 0 {
					draft.ActiveAccount = draft.Accounts[0].ID
				}
			}
			if e := s.saveConfigLocked(draft); e != nil {
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
