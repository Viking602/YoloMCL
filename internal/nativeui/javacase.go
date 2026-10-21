package nativeui

import "regexp"

var javaWord = regexp.MustCompile(`(?i)\bjava\b`)

// javaDisplay capitalises the word "java" in display-only text such as
// runtime folder names ("java-runtime-delta") or vendor strings. Never use
// it on paths or stored values.
func javaDisplay(s string) string { return javaWord.ReplaceAllString(s, "Java") }
