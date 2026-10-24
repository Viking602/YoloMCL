package textenc

import (
	"fmt"
	"strings"
	"testing"
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
	for range b.N {
		for _, l := range lines {
			_ = f(l)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(lines)), "ns/line")
}

func BenchmarkDecodeLineR10(b *testing.B) { benchLines(b, decodeLineR10) }
func BenchmarkDecodeLine(b *testing.B)    { benchLines(b, DecodeLine) }

func benchOne(b *testing.B, line []byte, f func([]byte) string) {
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	for range b.N {
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
