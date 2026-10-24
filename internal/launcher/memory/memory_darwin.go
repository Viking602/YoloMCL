//go:build darwin

package memory

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// physicalMemory reads hw.memsize via sysctl and estimates available memory
// from vm_stat (free + inactive + speculative pages).
func physicalMemory() (uint64, uint64, error) {
	raw, e := exec.Command("sysctl", "-n", "hw.memsize").Output()
	total, _ := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if e != nil || total == 0 {
		return 0, 0, errors.New("无法读取系统内存")
	}
	avail := total / 2
	if out, e := exec.Command("vm_stat").Output(); e == nil {
		page := uint64(4096)
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			if i := strings.Index(lines[0], "page size of "); i >= 0 {
				if n, e := strconv.ParseUint(strings.Fields(lines[0][i+13:])[0], 10, 64); e == nil {
					page = n
				}
			}
		}
		var pages uint64
		for _, l := range lines {
			k, v, ok := strings.Cut(l, ":")
			if !ok {
				continue
			}
			switch strings.TrimSpace(k) {
			case "Pages free", "Pages inactive", "Pages speculative":
				n, _ := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(v), "."), 10, 64)
				pages += n
			}
		}
		if pages > 0 {
			avail = min(total, pages*page)
		}
	}
	return total, avail, nil
}
