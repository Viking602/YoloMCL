//go:build darwin || linux

package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	keyring "github.com/zalando/go-keyring"
)

const keyringService = "YoloMCL account credentials"
const fileMagic = "YMV1"

var keyMu sync.Mutex

func secretPath(dir, id string) string {
	return filepath.Join(dir, "credentials", id+".bin")
}

// Each data directory has its own key, including copies of the same account.
func keyringAccount(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(sum[:])
}

func localKeyPath(dir string) string {
	return filepath.Join(dir, "credentials", "vault.key")
}

func readLocalKey(dir string) ([]byte, error) {
	path := localKeyPath(dir)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("本地登录密钥文件权限不安全")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("本地登录密钥文件无效")
	}
	return key, nil
}

func writePrivate(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mcl-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func masterKey(dir string, create bool) ([]byte, error) {
	keyMu.Lock()
	defer keyMu.Unlock()
	if runtime.GOOS == "linux" {
		key, err := readLocalKey(dir)
		if err == nil {
			return key, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	encoded, err := keyring.Get(keyringService, keyringAccount(dir))
	if runtime.GOOS == "linux" && err != nil && create {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := writePrivate(localKeyPath(dir), key); err != nil {
			return nil, err
		}
		return key, nil
	}
	if errors.Is(err, keyring.ErrNotFound) && create {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := keyring.Set(keyringService, keyringAccount(dir), base64.StdEncoding.EncodeToString(key)); err != nil {
			return nil, fmt.Errorf("系统凭据库不可用，无法保存登录状态: %w", err)
		}
		return key, nil
	}
	if runtime.GOOS == "linux" && err != nil {
		return nil, errors.New("旧登录密钥无法从 Secret Service 读取，请重新登录一次；新登录状态会自动保存在本机")
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, errors.New("登录凭据不可用，请重新登录")
	}
	if err != nil {
		return nil, fmt.Errorf("系统凭据库不可用，无法读取登录状态: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("系统凭据库中的登录密钥无效，请重新登录")
	}
	if runtime.GOOS == "linux" {
		if err := writePrivate(localKeyPath(dir), key); err != nil {
			return nil, err
		}
	}
	return key, nil
}

func Store(dir, id string, secret []byte) error {
	key, err := masterKey(dir, true)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	b := append([]byte(fileMagic), nonce...)
	b = aead.Seal(b, nonce, secret, []byte(id))
	return writePrivate(secretPath(dir, id), b)
}

func Load(dir, id string) ([]byte, error) {
	b, err := os.ReadFile(secretPath(dir, id))
	if err != nil {
		return nil, errors.New("登录凭据不可用，请重新登录")
	}
	key, err := masterKey(dir, false)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	start := len(fileMagic) + aead.NonceSize()
	if len(b) < start+aead.Overhead() || string(b[:len(fileMagic)]) != fileMagic {
		return nil, errors.New("登录凭据文件无效，请重新登录")
	}
	secret, err := aead.Open(nil, b[len(fileMagic):start], b[start:], []byte(id))
	if err != nil {
		return nil, errors.New("登录凭据文件校验失败，请重新登录")
	}
	return secret, nil
}

func Delete(dir, id string) error {
	err := os.Remove(secretPath(dir, id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
