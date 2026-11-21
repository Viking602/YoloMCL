//go:build windows

package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	Size uint32
	Data *byte
}

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var protect = crypt32.NewProc("CryptProtectData")
var unprotect = crypt32.NewProc("CryptUnprotectData")
var localFree = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")

func cryptBlob(b []byte, decrypt bool) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	in := dataBlob{uint32(len(b)), &b[0]}
	var out dataBlob
	proc := protect
	if decrypt {
		proc = unprotect
	}
	r, _, err := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(b)
	if r == 0 {
		return nil, fmt.Errorf("调用 Windows DPAPI 凭据操作失败: %w", err)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}

func secretPath(dir, id string) string {
	return filepath.Join(dir, "credentials", id+".bin")
}

// Store preserves the existing DPAPI-protected credential file format.
func Store(dir, id string, secret []byte) error {
	b, err := cryptBlob(secret, false)
	if err != nil {
		return err
	}
	path := secretPath(dir, id)
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

func Load(dir, id string) ([]byte, error) {
	b, err := os.ReadFile(secretPath(dir, id))
	if err != nil {
		return nil, errors.New("登录凭据不可用，请重新登录")
	}
	return cryptBlob(b, true)
}

func Delete(dir, id string) error {
	err := os.Remove(secretPath(dir, id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
