package launcher

import (
	"bytes"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// ansiDecoder is the system ANSI code page (Windows GetACP); Java before
// 18 writes stdout and latest.log in it, so a Chinese Windows log is GBK.
func ansiDecoder(cp uint32) encoding.Encoding {
	switch cp {
	case 936:
		return simplifiedchinese.GB18030
	case 950:
		return traditionalchinese.Big5
	case 932:
		return japanese.ShiftJIS
	case 949:
		return korean.EUCKR
	case 1250:
		return charmap.Windows1250
	case 1251:
		return charmap.Windows1251
	case 1252:
		return charmap.Windows1252
	}
	return nil
}

// DecodeLine turns one raw log line into readable text: valid UTF-8 stays
// as is; legacy text uses the system ANSI code page or GB18030. Recognizable
// GBK Chinese takes precedence over permissive Western single-byte decoders,
// so imported logs remain readable on English Windows. Invalid bytes are
// dropped, so the UI never shows replacement characters.
//
// The common case (ASCII or UTF-8) costs one scan plus the string copy;
// decoders are pooled per encoding instead of being built for every line.
func DecodeLine(b []byte) string {
	if utf8.Valid(b) {
		if bytes.IndexByte(b, 0xEF) >= 0 && bytes.Contains(b, replacementUTF8) {
			return string(bytes.ReplaceAll(b, replacementUTF8, nil))
		}
		return string(b)
	}
	return decodeLegacyLine(b, ansiEncoding())
}

func decodeLegacyLine(b []byte, system encoding.Encoding) string {
	// A single-byte code page accepts nearly every byte, including GBK. Do
	// not let that successful-but-garbled decode hide a run of Chinese text.
	// Require consecutive non-ASCII bytes, so isolated Western accents (for
	// example café or München) retain their system encoding.
	if system == charmap.Windows1250 || system == charmap.Windows1252 {
		run := 0
		for _, value := range b {
			if value >= 0x80 {
				run++
			} else {
				run = 0
			}
			if run >= 4 {
				if text, ok := decodeWith(simplifiedchinese.GB18030, b); ok {
					han := 0
					for _, r := range text {
						if unicode.Is(unicode.Han, r) {
							han++
						}
					}
					if han >= 2 {
						return text
					}
				}
				break
			}
		}
	}
	if s, ok := decodeWith(system, b); ok {
		return s
	}
	if s, ok := decodeWith(simplifiedchinese.GB18030, b); ok {
		return s
	}
	return strings.ToValidUTF8(string(b), "")
}

var replacementUTF8 = []byte("\uFFFD")

var (
	ansiOnce sync.Once
	ansiEnc  encoding.Encoding
	decPools sync.Map // encoding.Encoding -> *sync.Pool of *encoding.Decoder
)

func ansiEncoding() encoding.Encoding {
	ansiOnce.Do(func() { ansiEnc = ansiDecoder(systemCodePage()) })
	return ansiEnc
}

type pooledDecoder struct {
	d   *encoding.Decoder
	buf []byte
}

func decodeWith(enc encoding.Encoding, b []byte) (string, bool) {
	if enc == nil {
		return "", false
	}
	p, ok := decPools.Load(enc)
	if !ok {
		p, _ = decPools.LoadOrStore(enc, &sync.Pool{New: func() any { return &pooledDecoder{d: enc.NewDecoder()} }})
	}
	pool := p.(*sync.Pool)
	pd := pool.Get().(*pooledDecoder)
	// Legacy code pages grow at most 2x when decoded to UTF-8.
	if need := 2*len(b) + 16; cap(pd.buf) < need {
		pd.buf = make([]byte, need)
	}
	pd.d.Reset()
	n, _, e := pd.d.Transform(pd.buf[:cap(pd.buf)], b, true)
	out := pd.buf[:n]
	ok = e == nil && !bytes.Contains(out, replacementUTF8) && utf8.Valid(out)
	var str string
	if ok {
		str = string(out)
	}
	pool.Put(pd)
	return str, ok
}
