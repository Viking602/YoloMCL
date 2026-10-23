package nativeui

import "testing"

func TestParsePropsJavaEscapes(t *testing.T) {
	b := []byte("# Iris\r\nshaderPack=[\\u8D85\\u7F8E\\!\\u661F\\u7A7A]\\ Pack\\:v2.zip\r\nenableShaders=true\nlong=a\\\n  b\n")
	p := parseProps(b)
	if got := p["shaderPack"]; got != "[超美!星空] Pack:v2.zip" {
		t.Fatalf("shaderPack = %q", got)
	}
	if p["enableShaders"] != "true" || p["long"] != "ab" {
		t.Fatalf("props %q", p)
	}
	if got := propUnescape(`C:\\x\u00e9`); got != `C:\xé` {
		t.Fatalf("%q", got)
	}
}
