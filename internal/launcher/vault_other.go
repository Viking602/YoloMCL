//go:build !windows

package launcher

import "errors"

func seal(b []byte) ([]byte, error) {
	return nil, errors.New("此版本的安全凭据存储需要 Windows DPAPI")
}
func unseal(b []byte) ([]byte, error) {
	return nil, errors.New("此版本的安全凭据存储需要 Windows DPAPI")
}
