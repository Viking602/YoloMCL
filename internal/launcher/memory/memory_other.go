//go:build !windows && !darwin

package memory

import (
	"bytes"
	"errors"
	"os"
	"strconv"
)

func physicalMemory() (uint64, uint64, error) {
	b, e := os.ReadFile("/proc/meminfo")
	if e != nil {
		return 0, 0, errors.New("此系统暂不支持内存检测")
	}
	// Called on every snapshot poll: scan fields in place rather than
	// splitting the file and running a formatted scanner per line.
	var total, available uint64
	for len(b) > 0 && (total == 0 || available == 0) {
		line := b
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			line, b = b[:i], b[i+1:]
		} else {
			b = nil
		}
		key, rest, ok := bytes.Cut(line, []byte(":"))
		if !ok {
			continue
		}
		var dst *uint64
		switch string(key) {
		case "MemTotal":
			dst = &total
		case "MemAvailable":
			dst = &available
		default:
			continue
		}
		fields := bytes.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		if kb, err := strconv.ParseUint(string(fields[0]), 10, 64); err == nil {
			*dst = kb * 1024
		}
	}
	if total == 0 {
		return 0, 0, errors.New("无法读取系统内存")
	}
	return total, available, nil
}
