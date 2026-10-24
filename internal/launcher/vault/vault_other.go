//go:build !windows

package vault

import "errors"

func Seal(b []byte) ([]byte, error) {
	return nil, errors.New("此版本的安全凭据存储需要 Windows DPAPI")
}

func Unseal(b []byte) ([]byte, error) {
	return nil, errors.New("此版本的安全凭据存储需要 Windows DPAPI")
}
