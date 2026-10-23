package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestTailLinesMixedEncodings(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.log")
	var b []byte
	b = append(b, gbk(t, "第一行 GBK")...)
	b = append(b, "\r\n第二行 UTF-8\n"...)
	b = append(b, gbk(t, "第三行 GBK")...)
	b = append(b, '\n')
	if e := os.WriteFile(p, b, 0644); e != nil {
		t.Fatal(e)
	}
	lines, _, e := TailLines(p, 10, -1)
	if e != nil {
		t.Fatal(e)
	}
	want := []string{"第一行 GBK", "第二行 UTF-8", "第三行 GBK"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", lines)
	}
}

func TestGameLogDecodesStdout(t *testing.T) {
	l := &gameLog{ring: newLineRing(10)}
	l.Write(gbk(t, "保存中"))
	l.Write([]byte("\n"))
	if got := l.read(); len(got) != 1 || got[0] != "保存中" {
		t.Fatalf("%q", got)
	}
}

func TestZhNamesParse(t *testing.T) {
	z := newZhNames()
	z.parse([]byte("# mcmod.cn\n1708,apotheosis,apotheosis,神化,Apotheosis,,apotheosis\n17493,apotheosis-ascended,,Apotheosis Ascended,,,x\n3318,alexs-mobs,alexs-mobs,Alex 的生物,Alex's Mobs,AM,alexsmobs\n"), z.cf, z.mr, z.modID)
	if z.Mod("", "", "alexsmobs") != "Alex 的生物" || z.Mod("apotheosis", "", "") != "神化" || z.Mod("apotheosis-ascended", "", "") != "" {
		t.Fatalf("%v %v", z.cf, z.modID)
	}
	if z.Project(Project{Source: "modrinth", Type: "mod", URL: "https://modrinth.com/mod/alexs-mobs"}) != "Alex 的生物" {
		t.Fatal("project lookup")
	}
}
