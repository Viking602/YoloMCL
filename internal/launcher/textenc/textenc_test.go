package textenc

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func gbk(t *testing.T, s string) []byte {
	b, e := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s))
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestDecodeLineGBK(t *testing.T) {
	for _, s := range []string{"[30日8月2026 17:32:44.712] [Render thread/INFO] [fastquit/]: 正在保存世界 (1s)", "Disconnected \"已断开连接\" from the client.", "退出游戏"} {
		if got := DecodeLine(gbk(t, s)); got != s {
			t.Fatalf("GBK line: %q != %q", got, s)
		}
	}
	if got := DecodeLine([]byte("UTF-8 正常 ✓")); got != "UTF-8 正常 ✓" {
		t.Fatal(got)
	}
	if got := DecodeLine([]byte{'o', 'k', 0xff, 0xfe, 0x80}); strings.ContainsRune(got, '\uFFFD') || !strings.HasPrefix(got, "ok") {
		t.Fatalf("garbage bytes must not show U+FFFD: %q", got)
	}
}

func TestChineseLogsOnWesternWindowsKeepLatinAccents(t *testing.T) {
	for _, text := range []string{"正在保存世界", "退出游戏", "[INFO] 保存中"} {
		if got := decodeLegacyLine(gbk(t, text), charmap.Windows1252); got != text {
			t.Fatalf("imported Chinese log decoded using Western ANSI: %q", got)
		}
	}
	for _, text := range []string{"München", "café déjà vu", "€ 10", "naïve"} {
		data, err := charmap.Windows1252.NewEncoder().Bytes([]byte(text))
		if err != nil {
			t.Fatal(err)
		}
		if got := decodeLegacyLine(data, charmap.Windows1252); got != text {
			t.Fatalf("Western log changed: %q != %q", got, text)
		}
	}
}

func TestDecodeLineNoAllocASCII(t *testing.T) {
	line := []byte("[17:32:45] [Render thread/INFO]: plain ascii line")
	if n := testing.AllocsPerRun(1000, func() { _ = DecodeLine(line) }); n > 1 {
		t.Fatalf("ASCII line allocated %v times", n)
	}
	gbkBytes, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("成功保存"))
	if got := DecodeLine(gbkBytes); got != "成功保存" {
		t.Fatal(got)
	}
}
