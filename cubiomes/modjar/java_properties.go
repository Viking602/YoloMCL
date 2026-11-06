package modjar

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Properties.load(InputStream) uses ISO-8859-1 plus Java escapes, not UTF-8.
func readJavaProperties(raw []byte) (map[string]string, error) {
	if len(raw) > 2<<20 {
		return nil, fmt.Errorf("properties size limit")
	}
	decode := func(s string) (string, error) {
		var units []uint16
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c != '\\' {
				units = append(units, uint16(c))
				continue
			}
			i++
			if i == len(s) {
				break
			}
			switch c = s[i]; c {
			case 'u':
				if i+4 >= len(s) {
					return "", fmt.Errorf("truncated properties Unicode escape")
				}
				v, err := strconv.ParseUint(s[i+1:i+5], 16, 16)
				if err != nil {
					return "", fmt.Errorf("invalid properties Unicode escape")
				}
				units = append(units, uint16(v))
				i += 4
			case 'n':
				units = append(units, '\n')
			case 'r':
				units = append(units, '\r')
			case 't':
				units = append(units, '\t')
			case 'f':
				units = append(units, '\f')
			default:
				units = append(units, uint16(c))
			}
		}
		for i := 0; i < len(units); i++ {
			if units[i] >= 0xd800 && units[i] <= 0xdbff {
				if i+1 == len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return "", fmt.Errorf("unpaired properties surrogate")
				}
				i++
			} else if units[i] >= 0xdc00 && units[i] <= 0xdfff {
				return "", fmt.Errorf("unpaired properties surrogate")
			}
		}
		return string(utf16.Decode(units)), nil
	}
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n"), "\n")
	out := map[string]string{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimLeft(lines[i], " \t\f")
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		for {
			n := len(line) - len(strings.TrimRight(line, "\\"))
			if n%2 == 0 {
				break
			}
			line = line[:len(line)-1]
			if i+1 == len(lines) {
				break
			}
			i++
			line += strings.TrimLeft(lines[i], " \t\f")
		}
		end, escaped := 0, false
		for end < len(line) {
			if !escaped && strings.ContainsRune("=: \t\f", rune(line[end])) {
				break
			}
			if line[end] == '\\' {
				escaped = !escaped
			} else {
				escaped = false
			}
			end++
		}
		value := strings.TrimLeft(line[end:], " \t\f")
		if len(value) > 0 && (value[0] == '=' || value[0] == ':') {
			value = value[1:]
		}
		key, err := decode(line[:end])
		if err != nil {
			return nil, err
		}
		decoded, err := decode(strings.TrimLeft(value, " \t\f"))
		if err != nil {
			return nil, err
		}
		out[key] = decoded
	}
	return out, nil
}
