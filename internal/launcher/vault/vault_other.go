//go:build !windows && !darwin && !linux

package vault

import "errors"

var errUnsupported = errors.New("此平台尚不支持安全保存登录凭据")

func Store(string, string, []byte) error  { return errUnsupported }
func Load(string, string) ([]byte, error) { return nil, errUnsupported }
func Delete(string, string) error         { return errUnsupported }
