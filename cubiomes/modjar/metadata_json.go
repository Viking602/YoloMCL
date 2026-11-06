package modjar

import "bytes"

// Gson's quoted-string reader accepts literal control characters, an escaped
// newline and an escaped apostrophe even with lenient mode disabled. Preserve
// their values while making the text readable by encoding/json. This does not
// accept comments, unquoted keys or other lenient-mode extensions.
// Reference: FabricMC/fabric-loader JsonReader.nextQuotedValue/readEscapeCharacter.
func loaderJSON(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	out := make([]byte, 0, len(data))
	inString := false
	const hex = "0123456789abcdef"
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if inString && ch == '\\' && i+1 < len(data) {
			i++
			switch data[i] {
			case '\n':
				out = append(out, '\\', 'n')
			case '\'':
				out = append(out, '\'')
			default:
				out = append(out, ch, data[i])
			}
			continue
		}
		if ch == '"' {
			inString = !inString
		}
		if inString && ch < 32 {
			out = append(out, '\\', 'u', '0', '0', hex[ch>>4], hex[ch&15])
		} else {
			out = append(out, ch)
		}
	}
	return out
}
