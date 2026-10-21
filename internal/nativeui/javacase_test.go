package nativeui

import (
	"strings"
	"testing"
)

func TestJavaDisplayCapitalises(t *testing.T) {
	for in, want := range map[string]string{
		"java-runtime-delta": "Java-runtime-delta",
		"java-21-openjdk":    "Java-21-openjdk",
		"JAVA vendor":        "Java vendor",
		"javaw":              "javaw",
		"jdk-21.0.4+7":       "jdk-21.0.4+7",
		"Oracle java SE":     "Oracle Java SE",
	} {
		if got := javaDisplay(in); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestRecommendedJavaIsTag(t *testing.T) {
	_, tt := homeTester(t, "zh-CN")
	if _, ok := tt.Find("Java 标签 21"); !ok {
		t.Error("recommended Java should render as a tag")
	}
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, "· Java") {
			t.Errorf("inline Java text remains: %q", txt)
		}
	}
}
