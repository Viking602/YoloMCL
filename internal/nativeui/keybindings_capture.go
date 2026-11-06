package nativeui

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func parseBindingText(text string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(text))
	if strings.HasPrefix(value, "key.") || strings.HasPrefix(value, "scancode.") {
		base, modifier, has := strings.Cut(value, ":")
		if has {
			value = base + ":" + strings.ToUpper(modifier)
		}
		return launcher.NormalizeKeyBinding(value)
	}
	parts := strings.Split(value, "+")
	modifier := ""
	if len(parts) == 2 {
		switch strings.TrimSpace(parts[0]) {
		case "ctrl", "control":
			modifier = "CONTROL"
		case "shift":
			modifier = "SHIFT"
		case "alt":
			modifier = "ALT"
		default:
			return launcher.NormalizeKeyBinding("")
		}
		value = strings.TrimSpace(parts[1])
	} else if len(parts) > 2 {
		return launcher.NormalizeKeyBinding("")
	}
	aliases := map[string]string{"esc": "escape", "spacebar": "space", "ctrl": "left.control", "shift": "left.shift", "alt": "left.alt", "none": "unknown", "unbound": "unknown", "mouse1": "mouse.left", "mouse2": "mouse.right", "mouse3": "mouse.middle", "mouse4": "mouse.4", "mouse5": "mouse.5"}
	if v, ok := aliases[value]; ok {
		value = v
	}
	if strings.HasPrefix(value, "mouse.") {
		value = "key." + value
	} else {
		value = "key.keyboard." + value
	}
	if modifier != "" {
		value += ":" + modifier
	}
	return launcher.NormalizeKeyBinding(value)
}

func captureKeyName(key ui.Key) string {
	switch {
	case key >= ui.KeyA && key <= ui.KeyZ:
		return string(rune('a' + key - ui.KeyA))
	case key >= ui.Key0 && key <= ui.Key9:
		return string(rune('0' + key - ui.Key0))
	case key >= ui.KeyF1 && key <= ui.KeyF12:
		return "f" + strconv.Itoa(int(key-ui.KeyF1)+1)
	}
	return map[ui.Key]string{
		ui.KeyEnter: "enter", ui.KeyEscape: "escape", ui.KeyBackspace: "backspace", ui.KeyTab: "tab", ui.KeySpace: "space", ui.KeyDelete: "delete", ui.KeyInsert: "insert",
		ui.KeyHome: "home", ui.KeyEnd: "end", ui.KeyPageUp: "page.up", ui.KeyPageDown: "page.down", ui.KeyLeft: "left", ui.KeyRight: "right", ui.KeyUp: "up", ui.KeyDown: "down",
		ui.KeyMinus: "minus", ui.KeyEqual: "equal", ui.KeyComma: "comma", ui.KeyPeriod: "period", ui.KeySlash: "slash", ui.KeySemicolon: "semicolon", ui.KeyQuote: "apostrophe",
		ui.KeyBracketLeft: "left.bracket", ui.KeyBracketRight: "right.bracket", ui.KeyBackslash: "backslash", ui.KeyBackquote: "grave.accent", ui.KeyContextMenu: "menu",
	}[key]
}

// Capture is local to the focused key button. Escape cancels in dialogView;
// pointer buttons are captured only when pressed on the recording control.
func (a *App) captureBinding(g *keyBindingsState, row launcher.KeyBindingRow, ev ui.InputEvent) bool {
	if a.keyBindings != g || g.capture != row.ID || a.dialogPending() {
		return false
	}
	value := ""
	switch ev.Kind {
	case ui.InputKeyDown:
		key := ev.PhysicalKey
		if key == ui.KeyUnknown {
			key = ev.Key
		}
		if key == ui.KeyEscape {
			g.capture = ""
			return true
		}
		if key == ui.KeyBack {
			value = "key.mouse.4"
		} else if key == ui.KeyForward {
			value = "key.mouse.5"
		} else if name := captureKeyName(key); name != "" {
			value = "key.keyboard." + name
		}
	case ui.InputPointerDown:
		if ev.Button >= 0 && ev.Button < 8 {
			names := []string{"left", "right", "middle"}
			name := strconv.Itoa(ev.Button + 1)
			if ev.Button < len(names) {
				name = names[ev.Button]
			}
			value = "key.mouse." + name
		}
	case ui.InputKeyUp, ui.InputText, ui.InputCompose:
		return true
	default:
		return false
	}
	if value == "" {
		return true
	}
	modifier := ""
	switch ev.Mods {
	case 0:
	case ui.Ctrl:
		modifier = "CONTROL"
	case ui.Shift:
		modifier = "SHIFT"
	case ui.Alt:
		modifier = "ALT"
	default:
		a.notice(a.L("game_options.single_modifier"))
		return true
	}
	if modifier != "" {
		if g.rootID == "" || !g.view.Modifiers && !strings.Contains(row.Current, ":") {
			a.notice(a.L("game_options.modifier_unsupported"))
			return true
		}
		value += ":" + modifier
	}
	g.bindings[row.ID] = value
	delete(g.inputs, row.ID)
	delete(g.invalid, row.ID)
	g.capture = ""
	return true
}
