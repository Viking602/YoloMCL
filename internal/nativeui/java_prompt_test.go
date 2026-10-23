package nativeui

import "testing"

func TestNeedJavaPrompt(t *testing.T) {
	m := needJavaRE.FindStringSubmatch("需要 Java 21，还没有安装")
	if m == nil || m[1] != "21" {
		t.Fatal(m)
	}
}
