package launcher

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// mixedLog is a realistic latest.log: mostly ASCII, some UTF-8 Chinese,
// some GBK lines from the JVM's console encoding.
func mixedLog(n int) [][]byte {
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("[17:32:45] [Server thread/INFO] [fastquit]: 成功保存\"新的世界\"(1s)"))
	out := make([][]byte, n)
	for i := range out {
		switch i % 10 {
		case 0:
			out[i] = gbk
		case 1:
			out[i] = []byte("[17:32:45] [Render thread/INFO] [minecraft/Minecraft]: 正在加载资源包 逆转未来 2.3.3")
		default:
			out[i] = []byte(fmt.Sprintf("[17:32:45] [Worker-Main-%d/INFO] [epicfight]: Data keys [efn:charge_power, efn:charge_ticks, efn:key2_press_time]", i%7))
		}
	}
	return out
}

func benchLines(b *testing.B, f func([]byte) string) {
	lines := mixedLog(120000)
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	b.SetBytes(int64(total))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, l := range lines {
			_ = f(l)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(lines)), "ns/line")
}

func BenchmarkDecodeLineR10(b *testing.B) { benchLines(b, decodeLineR10) }
func BenchmarkDecodeLine(b *testing.B)    { benchLines(b, DecodeLine) }

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
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < b.N; i++ {
		if _, _, e := TailLines(p, 7000, -1); e != nil {
			b.Fatal(e)
		}
	}
}

func TestDecodeLineNoAllocASCII(t *testing.T) {
	line := []byte("[17:32:45] [Render thread/INFO]: plain ascii line")
	if n := testing.AllocsPerRun(1000, func() { _ = DecodeLine(line) }); n > 1 {
		t.Fatalf("ASCII line allocated %v times", n)
	}
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("成功保存"))
	if got := DecodeLine(gbk); got != "成功保存" {
		t.Fatal(got)
	}
}

func benchOne(b *testing.B, line []byte, f func([]byte) string) {
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = f(line)
	}
}

var asciiLine = []byte("[17:32:45] [Worker-Main-3/INFO] [epicfight]: Data keys [efn:charge_power, efn:charge_ticks, efn:key2_press_time]")

func gbkLine() []byte {
	g, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("[17:32:45] [Server thread/INFO] [fastquit]: 成功保存\"新的世界\"(1s)"))
	return g
}

func BenchmarkASCIIR10(b *testing.B) { benchOne(b, asciiLine, decodeLineR10) }
func BenchmarkASCII(b *testing.B)    { benchOne(b, asciiLine, DecodeLine) }
func BenchmarkGBKR10(b *testing.B)   { benchOne(b, gbkLine(), decodeLineR10) }
func BenchmarkGBK(b *testing.B)      { benchOne(b, gbkLine(), DecodeLine) }
