package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLineRingKeepsLatest(t *testing.T) {
	r := newLineRing(3)
	for i := 0; i < 10; i++ {
		r.Add(fmt.Sprint(i))
	}
	if got := strings.Join(r.Lines(), ","); got != "7,8,9" || r.Dropped() != 7 {
		t.Fatalf("ring = %q dropped %d", got, r.Dropped())
	}
	r2 := newLineRing(5)
	r2.Add("a")
	r2.Add("b")
	if got := strings.Join(r2.Lines(), ","); got != "a,b" || r2.Dropped() != 0 {
		t.Fatalf("partial ring = %q", got)
	}
}

func TestGameLogIsBounded(t *testing.T) {
	l := &gameLog{ring: newLineRing(100), secret: "tok"}
	for i := 0; i < 1000; i++ {
		_, _ = l.Write([]byte(fmt.Sprintf("line %d tok\r\n", i)))
	}
	lines, dropped := l.snapshot()
	if len(lines) != 100 || dropped != 900 || lines[99] != "line 999 [REDACTED]" {
		t.Fatalf("got %d lines, dropped %d, last %q", len(lines), dropped, lines[len(lines)-1])
	}
}

func TestTailLinesLargeFileCRLFAndPaging(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.log")
	var b strings.Builder
	const total = 200000 // ~6 MB: far more than one chunk
	for i := 0; i < total; i++ {
		fmt.Fprintf(&b, "[12:00:00] [main/INFO]: line %06d 中文\r\n", i)
	}
	if e := os.WriteFile(p, []byte(b.String()), 0644); e != nil {
		t.Fatal(e)
	}
	lines, start, e := TailLines(p, 5000, -1)
	if e != nil || len(lines) != 5000 {
		t.Fatalf("tail: %d lines, %v", len(lines), e)
	}
	if !strings.HasSuffix(lines[4999], "line 199999 中文") || strings.Contains(lines[0], "\r") || !strings.Contains(lines[0], "line 195000") {
		t.Fatalf("tail content: first %q last %q", lines[0], lines[4999])
	}
	older, start2, e := TailLines(p, 5000, start)
	if e != nil || len(older) != 5000 || !strings.Contains(older[4999], "line 194999") || start2 >= start {
		t.Fatalf("paging: %d %q %d→%d %v", len(older), older[len(older)-1], start, start2, e)
	}
	// Reaching the beginning returns start 0 and the first line.
	first, s0, _ := TailLines(p, 10, 200)
	if s0 != 0 || !strings.Contains(first[0], "line 000000") {
		t.Fatalf("head page: %q start %d", first, s0)
	}
}

func TestTailLinesInvalidBytesAreSafe(t *testing.T) {
	p := filepath.Join(t.TempDir(), "gbk.log")
	gbk := []byte{0xd6, 0xd0, 0xce, 0xc4} // "中文" in GBK: invalid UTF-8
	data := append([]byte("ok\n"), append(gbk, []byte("\nlast")...)...)
	if e := os.WriteFile(p, data, 0644); e != nil {
		t.Fatal(e)
	}
	lines, _, e := TailLines(p, 10, -1)
	if e != nil || len(lines) != 3 || lines[2] != "last" {
		t.Fatalf("%q %v", lines, e)
	}
	for _, l := range lines {
		if !utf8.ValidString(l) {
			t.Fatalf("invalid utf-8 leaked: %q", l)
		}
	}
}
