package launcher

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func gbk(t *testing.T, s string) []byte {
	b, e := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s))
	if e != nil {
		t.Fatal(e)
	}
	return b
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

// gameLogR10 is the r10 stream path, kept for the benchmark.
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

func decodeLineR10(b []byte) string {
	if utf8.Valid(b) {
		return strings.ReplaceAll(string(b), "\uFFFD", "")
	}
	if s, e := simplifiedchinese.GB18030.NewDecoder().Bytes(b); e == nil && utf8.Valid(s) && !strings.ContainsRune(string(s), '\uFFFD') {
		return string(s)
	}
	return strings.ToValidUTF8(string(b), "")
}

func mixedLog(n int) [][]byte {
	gbkBytes, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("[17:32:45] [Server thread/INFO] [fastquit]: 成功保存\"新的世界\"(1s)"))
	out := make([][]byte, n)
	for i := range out {
		switch i % 10 {
		case 0:
			out[i] = gbkBytes
		case 1:
			out[i] = []byte("[17:32:45] [Render thread/INFO] [minecraft/Minecraft]: 正在加载资源包 逆转未来 2.3.3")
		default:
			out[i] = []byte(fmt.Sprintf("[17:32:45] [Worker-Main-%d/INFO] [epicfight]: Data keys [efn:charge_power, efn:charge_ticks, efn:key2_press_time]", i%7))
		}
	}
	return out
}

func streamBytes() []byte {
	var buf bytes.Buffer
	for _, l := range mixedLog(120000) {
		buf.Write(l)
		buf.WriteString("\r\n")
	}
	return buf.Bytes()
}

// Java flushes the console in ~8 KiB writes.
func benchStream(b *testing.B, write func([]byte)) {
	data := streamBytes()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for off := 0; off < len(data); off += 8192 {
			write(data[off:min(off+8192, len(data))])
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*120000), "ns/line")
}

func BenchmarkStreamR10(b *testing.B) {
	l := &gameLogR10{ring: newLineRing(LogRingLines)}
	benchStream(b, l.Write)
}

func BenchmarkStream(b *testing.B) {
	l := &gameLog{ring: newLineRing(LogRingLines), secret: "tok"}
	benchStream(b, func(p []byte) { _, _ = l.Write(p) })
}

// TailLines on a 120k-line file only decodes the requested window.
func BenchmarkTailLines(b *testing.B) {
	p := filepath.Join(b.TempDir(), "latest.log")
	if e := os.WriteFile(p, streamBytes(), 0644); e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, _, e := TailLines(p, 7000, -1); e != nil {
			b.Fatal(e)
		}
	}
}
