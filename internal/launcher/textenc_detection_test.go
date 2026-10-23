package launcher

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// decodeLineR10 is the r10 implementation, kept for the benchmark.
func decodeLineR10(b []byte) string {
	if utf8.Valid(b) {
		return strings.ReplaceAll(string(b), "\uFFFD", "")
	}
	for _, enc := range []encoding.Encoding{ansiDecoder(systemCodePage()), simplifiedchinese.GB18030} {
		if enc == nil {
			continue
		}
		if s, e := enc.NewDecoder().Bytes(b); e == nil && utf8.Valid(s) && !strings.ContainsRune(string(s), '\uFFFD') {
			return string(s)
		}
	}
	return strings.ToValidUTF8(string(b), "")
}

// gameLogWriteR10 is the r10 stream path, kept for the benchmark.
type gameLogR10 struct {
	ring    *lineRing
	pending string
}

func (l *gameLogR10) Write(b []byte) {
	text := append([]byte(l.pending), b...)
	parts := bytes.Split(text, []byte("\n"))
	l.pending = string(parts[len(parts)-1])
	for _, raw := range parts[:len(parts)-1] {
		v := decodeLineR10(bytes.TrimRight(raw, "\r"))
		v = strings.TrimRight(v, "\r")
		l.ring.Add(v)
	}
}
