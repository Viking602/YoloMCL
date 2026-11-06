//go:build !windows

package launcher

import (
	"os"
	"strings"
)

// hostJavaLanguage is the language a JVM started by this launcher takes as its
// default locale: on POSIX, the LC_ALL / LC_CTYPE / LANG environment.
func hostJavaLanguage() string {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); v != "" && v != "C" && v != "POSIX" {
			lang, _, _ := strings.Cut(v, "_")
			lang, _, _ = strings.Cut(lang, ".")
			return strings.ToLower(lang)
		}
	}
	return ""
}
