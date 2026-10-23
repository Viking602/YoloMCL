package launcher

import "strings"

// NormalizeLanguage returns a supported, explicitly chosen UI language.
// Empty, auto, system and unsupported values follow the system instead.
// Accept both BCP 47 tags and Unix locale names without guessing from a
// country, timezone or keyboard layout.
func NormalizeLanguage(lang string) string {
	v := strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(v, "-_.@"); i >= 0 {
		v = v[:i]
	}
	switch v {
	case "zh":
		return "zh-CN"
	case "en":
		return "en-US"
	default:
		return ""
	}
}

// SystemLanguage picks the first supported language in the OS preference
// order. Chinese variants use the available Simplified Chinese translation;
// systems without a supported preference fall back to English.
func SystemLanguage(preferred []string) string {
	for _, tag := range preferred {
		if lang := NormalizeLanguage(tag); lang != "" {
			return lang
		}
	}
	return "en-US"
}
