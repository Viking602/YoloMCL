package i18n

import (
	"regexp"
	"strings"
)

var (
	unitPair = regexp.MustCompile(`([0-9])[ \t]+([\p{L}°%]+(?:/[\p{L}]+)?)`)
	dashPair = regexp.MustCompile(`—[ \t]+([\p{L}])`)
)

// BindTextPairs keeps numbers with recognized units and a dash with the next
// name. It only formats rendered prose; catalogs and stored input stay intact.
func BindTextPairs(text string) string {
	text = unitPair.ReplaceAllStringFunc(text, func(pair string) string {
		parts := unitPair.FindStringSubmatch(pair)
		unit := parts[2]
		known := false
		switch unit {
		case "B", "KB", "MB", "GB", "TB", "KiB", "MiB", "GiB", "TiB",
			"B/s", "KB/s", "MB/s", "GB/s", "KiB/s", "MiB/s", "GiB/s",
			"ms", "s", "min", "h", "Hz", "kHz", "MHz", "GHz", "fps", "%", "°C",
			"mm", "cm", "m", "km", "px", "DIP", "DIPs",
			"second", "seconds", "minute", "minutes", "hour", "hours",
			"file", "files", "block", "blocks", "chunk", "chunks":
			known = true
		}
		for _, prefix := range []string{"毫秒", "秒", "分钟", "小时", "字节", "格", "个", "块"} {
			known = known || strings.HasPrefix(unit, prefix)
		}
		if known {
			return parts[1] + "\u00a0" + unit
		}
		return pair
	})
	return dashPair.ReplaceAllString(text, "—\u00a0${1}")
}
