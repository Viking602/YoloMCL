//go:build !windows && !darwin

package memory

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func physicalMemory() (uint64, uint64, error) {
	b, e := os.ReadFile("/proc/meminfo")
	if e != nil {
		return 0, 0, errors.New("此系统暂不支持内存检测")
	}
	var total, available uint64
	for _, line := range strings.Split(string(b), "\n") {
		var key string
		var kb uint64
		if _, e = fmt.Sscanf(line, "%s %d", &key, &kb); e == nil {
			switch key {
			case "MemTotal:":
				total = kb * 1024
			case "MemAvailable:":
				available = kb * 1024
			}
		}
	}
	if total == 0 {
		return 0, 0, errors.New("无法读取系统内存")
	}
	return total, available, nil
}
