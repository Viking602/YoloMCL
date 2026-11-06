package launcher

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

//go:embed keybindings_catalog.json
var keyBindingCatalogJSON []byte

type vanillaKeyBinding struct {
	Default string   `json:"default"`
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Aliases []string `json:"aliases,omitempty"`
}

var keyBindingCatalog = func() struct {
	Vanilla []vanillaKeyBinding `json:"vanilla"`
	Legacy  map[string]string   `json:"legacy"`
} {
	var c struct {
		Vanilla []vanillaKeyBinding `json:"vanilla"`
		Legacy  map[string]string   `json:"legacy"`
	}
	if err := json.Unmarshal(keyBindingCatalogJSON, &c); err != nil {
		panic(err)
	}
	return c
}()

func vanillaBinding(key string) (vanillaKeyBinding, bool) {
	for _, b := range keyBindingCatalog.Vanilla {
		if key == b.ID || slices.Contains(b.Aliases, key) {
			return b, true
		}
	}
	return vanillaKeyBinding{}, false
}

func keyBindingError(key string, args ...any) error {
	return &bindingError{key: key, args: args}
}

type bindingError struct {
	key  string
	args []any
}

func (e *bindingError) Error() string { return e.LocalizedMessage(false) }
func (e *bindingError) LocalizedMessage(en bool) string {
	return fmt.Sprintf(i18n.Text(en, "keybindings.error."+e.key), e.args...)
}

func validBindingID(key string) bool {
	return strings.HasPrefix(key, "key_") && len(key) > 4 && len(key) <= 256 && !strings.ContainsAny(key, ":\r\n\x00") && strings.IndexFunc(key, unicode.IsControl) < 0
}

// NormalizeKeyBinding translates LWJGL 2 codes to Minecraft key names. The
// numeric mapping is data in the catalog, based on LWJGL's Keyboard constants:
// https://github.com/LWJGL/lwjgl/blob/master/src/java/org/lwjgl/input/Keyboard.java
// Forge's optional modifier suffix belongs to the value, not another option.
func NormalizeKeyBinding(value string) (string, error) {
	base, modifier, hasModifier := strings.Cut(strings.TrimSpace(value), ":")
	if hasModifier && modifier != "NONE" && modifier != "SHIFT" && modifier != "CONTROL" && modifier != "ALT" {
		return "", keyBindingError("invalid_binding", value)
	}
	if n, err := strconv.Atoi(base); err == nil {
		if n >= -100 && n < 0 {
			i := n + 100
			base = "key.mouse." + strconv.Itoa(i+1)
			if i < 3 {
				base = []string{"key.mouse.left", "key.mouse.right", "key.mouse.middle"}[i]
			}
		} else if name, ok := keyBindingCatalog.Legacy[base]; ok {
			base = "key.keyboard." + name
		} else {
			return "", keyBindingError("invalid_binding", value)
		}
	}
	valid := false
	for _, prefix := range []string{"key.keyboard.", "key.mouse.", "scancode."} {
		if strings.HasPrefix(base, prefix) && len(base) > len(prefix) {
			valid = true
		}
	}
	if !valid || len(base) > 100 || strings.IndexFunc(base, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-')
	}) >= 0 {
		return "", keyBindingError("invalid_binding", value)
	}
	if hasModifier && modifier != "NONE" && base != "key.keyboard.unknown" {
		base += ":" + modifier
	}
	return base, nil
}

func bindingForOptions(value, current string) (string, error) {
	value, err := NormalizeKeyBinding(value)
	if err != nil {
		return "", err
	}
	oldBase, _, _ := strings.Cut(current, ":")
	if _, err := strconv.Atoi(oldBase); err != nil {
		return value, nil
	}
	base, modifier, hasModifier := strings.Cut(value, ":")
	code := ""
	for number, name := range keyBindingCatalog.Legacy {
		if base == "key.keyboard."+name {
			code = number
			break
		}
	}
	if strings.HasPrefix(base, "key.mouse.") {
		name := strings.TrimPrefix(base, "key.mouse.")
		n, _ := strconv.Atoi(name)
		switch name {
		case "left":
			n = 1
		case "right":
			n = 2
		case "middle":
			n = 3
		}
		if n > 0 && n <= 100 {
			code = strconv.Itoa(n - 101)
		}
	}
	if code == "" {
		return "", keyBindingError("legacy_binding", value)
	}
	if hasModifier {
		code += ":" + modifier
	}
	return code, nil
}

// KeyBindingChoices uses the same canonical names as persisted profiles.
func KeyBindingChoices() []string {
	keys := []string{"key.keyboard.unknown", "key.mouse.left", "key.mouse.right", "key.mouse.middle", "key.mouse.4", "key.mouse.5"}
	var keyboard []string
	for _, name := range keyBindingCatalog.Legacy {
		if name != "unknown" {
			keyboard = append(keyboard, "key.keyboard."+name)
		}
	}
	slices.Sort(keyboard)
	return append(keys, keyboard...)
}
