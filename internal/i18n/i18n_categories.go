package i18n

import "strings"

// TranslateCategory resolves provider slugs and names from the locale catalog.
// Unknown categories retain their original spelling and capitalization.
func TranslateCategory(en bool, cat string) string {
	raw := strings.TrimSpace(cat)
	if text, ok := forLanguage(en).Categories[strings.ToLower(raw)]; ok {
		return text
	}
	return raw
}
