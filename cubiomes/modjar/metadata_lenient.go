package modjar

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Legacy FML feeds mcmod.info through Gson JsonParser's lenient JsonReader.
// Keep this grammar separate from Fabric/Quilt's metadata contract.
func legacyJSON(data []byte) (out any, err error) {
	defer func() {
		if p := recover(); p != nil {
			out, err = nil, fmt.Errorf("legacy metadata: %v", p)
		}
	}()
	if len(data) > 2<<20 {
		return nil, fmt.Errorf("legacy metadata size limit")
	}
	p := legacyJSONReader{data: strings.TrimPrefix(string(data), "\xef\xbb\xbf"), nodes: 131072}
	p.space()
	if strings.HasPrefix(p.data[p.pos:], ")]}'\n") {
		p.pos += 5
	}
	out = p.value(0)
	p.space()
	if p.pos != len(p.data) {
		fail("trailing metadata content")
	}
	return out, nil
}

type legacyJSONReader struct {
	data       string
	pos, nodes int
}

func (p *legacyJSONReader) space() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\r', '\n', '\f':
			p.pos++
		case '#':
			for p.pos < len(p.data) && p.data[p.pos] != '\n' && p.data[p.pos] != '\r' {
				p.pos++
			}
		case '/':
			if strings.HasPrefix(p.data[p.pos:], "//") {
				p.pos += 2
				for p.pos < len(p.data) && p.data[p.pos] != '\n' && p.data[p.pos] != '\r' {
					p.pos++
				}
			} else if strings.HasPrefix(p.data[p.pos:], "/*") {
				n := strings.Index(p.data[p.pos+2:], "*/")
				if n < 0 {
					fail("unterminated metadata comment")
				}
				p.pos += n + 4
			} else {
				return
			}
		default:
			return
		}
	}
}

func (p *legacyJSONReader) take() byte {
	if p.pos >= len(p.data) {
		fail("unexpected end of metadata")
	}
	c := p.data[p.pos]
	p.pos++
	return c
}

func (p *legacyJSONReader) token() string {
	p.space()
	if p.pos >= len(p.data) {
		fail("missing metadata value")
	}
	if q := p.data[p.pos]; q == '"' || q == '\'' {
		p.pos++
		var encoded []byte
		encoded = append(encoded, '"')
		for {
			c := p.take()
			if c == q {
				break
			}
			if c == '\\' {
				n := p.take()
				if n == '\'' {
					encoded = append(encoded, '\'')
				} else if n == '\n' || n == '\r' {
					b, _ := json.Marshal(string(n))
					encoded = append(encoded, b[1:len(b)-1]...)
				} else {
					encoded = append(encoded, '\\', n)
				}
			} else if c == '"' {
				encoded = append(encoded, '\\', c)
			} else if c < 32 {
				b, _ := json.Marshal(string(c))
				encoded = append(encoded, b[1:len(b)-1]...)
			} else {
				encoded = append(encoded, c)
			}
		}
		encoded = append(encoded, '"')
		var out string
		if json.Unmarshal(encoded, &out) != nil {
			fail("invalid metadata string escape")
		}
		return out
	}
	start := p.pos
	for p.pos < len(p.data) && !strings.ContainsRune("{}[]:,;=\\/# \t\r\n\f", rune(p.data[p.pos])) {
		p.pos++
	}
	if p.pos == start {
		fail("invalid metadata token")
	}
	return p.data[start:p.pos]
}

func (p *legacyJSONReader) value(depth int) any {
	p.nodes--
	if depth > 64 || p.nodes < 0 {
		fail("metadata nesting or value budget")
	}
	p.space()
	if p.pos == len(p.data) {
		fail("missing metadata value")
	}
	switch p.data[p.pos] {
	case '{':
		p.pos++
		p.space()
		out := map[string]any{}
		if p.pos < len(p.data) && p.data[p.pos] == '}' {
			p.pos++
			return out
		}
		for {
			key := p.token()
			p.space()
			separator := p.take()
			if separator == '=' {
				if p.pos < len(p.data) && p.data[p.pos] == '>' {
					p.pos++
				}
			} else if separator != ':' {
				fail("missing metadata member separator")
			}
			out[key] = p.value(depth + 1)
			p.space()
			switch p.take() {
			case '}':
				return out
			case ',', ';':
			default:
				fail("invalid metadata object separator")
			}
		}
	case '[':
		p.pos++
		p.space()
		out := []any{}
		if p.pos < len(p.data) && p.data[p.pos] == ']' {
			p.pos++
			return out
		}
		for {
			p.space()
			if p.pos < len(p.data) && strings.ContainsRune(",;]", rune(p.data[p.pos])) {
				p.nodes--
				out = append(out, nil)
			} else {
				out = append(out, p.value(depth+1))
			}
			if p.nodes < 0 {
				fail("metadata value budget")
			}
			p.space()
			switch p.take() {
			case ']':
				return out
			case ',', ';':
			default:
				fail("invalid metadata array separator")
			}
		}
	case '"', '\'':
		return p.token()
	default:
		token := p.token()
		switch strings.ToLower(token) {
		case "true":
			return true
		case "false":
			return false
		case "null":
			return nil
		}
		if (token[0] == '-' || token[0] >= '0' && token[0] <= '9') && json.Valid([]byte(token)) {
			return json.Number(token)
		}
		return token
	}
}
