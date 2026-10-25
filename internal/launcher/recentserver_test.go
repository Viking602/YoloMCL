package launcher

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func serverFromLog(ctx context.Context, data []byte, file string, modified time.Time) (joined, attempted *InstanceServer) {
	state := serverLogState{file: file, modified: modified}
	for raw := range bytes.Lines(data) {
		if ctx.Err() != nil {
			return nil, nil
		}
		state.line(raw)
	}
	return state.joined, state.attempted
}

func TestRecentServerRecognizesJoinsAndFailedAttempts(t *testing.T) {
	stamp := time.Date(2026, 10, 22, 23, 0, 0, 0, time.Local)
	data := []byte("[19:01:02] [Render thread/INFO] [minecraft/ConnectScreen]: Connecting to play.example.test, 25565\n" +
		"[19:01:05] [Render thread/INFO] [minecraft/ClientPacketListener]: Loaded 68 advancements\n" +
		"[20:00:00] [Render thread/INFO]: Connecting to offline.example.test, 25566\n" +
		"[20:00:03] [Render thread/ERROR]: Couldn't connect to server\n" +
		"[21:00:00] [Render thread/INFO]: Loaded 80 advancements\n" +
		"[21:00:10] [Render thread/INFO]: [CHAT] Connecting to forged.example.test, 25565\n")
	joined, attempt := serverFromLog(context.Background(), data, "latest.log", stamp)
	if joined == nil || !joined.Confirmed || joined.Address != "play.example.test:25565" || joined.Connected.Hour() != 19 {
		t.Fatalf("wrong successful join: %+v", joined)
	}
	if attempt == nil || attempt.Confirmed || attempt.Address != "offline.example.test:25566" {
		t.Fatalf("failed attempt was treated as a successful join: %+v", attempt)
	}
	joined, _ = serverFromLog(context.Background(), []byte("[22:00:00] [Client thread/INFO]: Connecting to 2001:db8::1, 25570\n[22:00:01] [Netty Client IO/INFO] [FML]: Connected to a modded server.\n"), "latest.log", stamp)
	if joined == nil || joined.Address != "[2001:db8::1]:25570" {
		t.Fatalf("IPv6/modded server not recognized: %+v", joined)
	}
}

func TestRecentServerUsesArchivedLogsAndInstanceScope(t *testing.T) {
	s, r, _ := worldFixture(t)
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte("[22:30:00] [Render thread/INFO]: Connecting to friends.example.test, 25565\n[22:30:02] [Render thread/INFO]: Loaded 5 advancements\n"))
	_ = w.Close()
	old := filepath.Join(r.Path, "logs", "2026-10-21-1.log.gz")
	writeTestFile(t, old, gz.Bytes())
	stamp := time.Date(2026, 10, 21, 23, 0, 0, 0, time.Local)
	if err := os.Chtimes(old, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(r.Path, "logs", "latest.log"), []byte("[10:00:00] [Render thread/INFO]: Loaded 4 advancements\n"))
	// A saved favourite and another isolated instance are not connection history.
	writeTestFile(t, filepath.Join(r.Path, "servers.dat"), []byte("favourite.example.test"))
	writeTestFile(t, filepath.Join(r.Path, "versions", "other", "logs", "latest.log"), []byte("[23:30:00] [Render thread/INFO]: Connecting to wrong.example.test, 25565\n[23:30:01] [Render thread/INFO]: Loaded 5 advancements\n"))
	st, err := s.InstanceStatus(r.ID, "")
	if err != nil || st.RecentServer == nil || st.RecentServer.Address != "friends.example.test:25565" || st.RecentServer.Connected.Day() != 21 {
		t.Fatalf("wrong server for this instance: %+v, %v", st.RecentServer, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = readRecentServer(ctx, r.Path, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("obsolete server scan did not stop: %v", err)
	}
}

func TestRecentServerReadsAreBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest.log")
	tail := "[22:00:00] [Render thread/INFO]: Connecting to tail.example.test, 25565\n[22:00:01] [Render thread/INFO]: Loaded 3 advancements\n"
	writeTestFile(t, path, []byte(strings.Repeat("old log data\n", 10000)+tail))
	joined, _, read := readServerLog(context.Background(), path, time.Now(), 1024)
	if read > 1024 || joined == nil || joined.Address != "tail.example.test:25565" {
		t.Fatal("large plain log was not read from its bounded tail")
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(strings.Repeat("log\n", 10000)))
	_ = w.Close()
	writeTestFile(t, path+".gz", gz.Bytes())
	if joined, attempt, read := readServerLog(context.Background(), path+".gz", time.Now(), 1024); joined != nil || attempt != nil || read <= 1024 {
		t.Fatal("truncated archive could misidentify an older connection as latest")
	}
}

func TestRecentServerStreamKeepsNewestJoinAndSkipsLongLines(t *testing.T) {
	join := func(host string) string {
		return "[20:00:00] [Render thread/INFO]: Connecting to " + host + ", 25565\n" +
			"[20:00:01] [Render thread/INFO]: Loaded 3 advancements\n"
	}
	// An oversized line must not be parsed as independent fragments, including
	// a connection marker exactly at the next buffer boundary.
	data := []byte(join("older.example.test") + strings.Repeat("x", serverLogLineSize*3) + join("fragment.example.test") +
		join("newest.example.test") + "[20:01:00] [Render thread/INFO]: Connecting to pending.example.test, 25565")
	for _, compressed := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "latest.log")
		content := data
		if compressed {
			var gz bytes.Buffer
			w := gzip.NewWriter(&gz)
			_, _ = w.Write(data)
			_ = w.Close()
			content, path = gz.Bytes(), path+".gz"
		}
		writeTestFile(t, path, content)
		joined, attempted, read := readServerLog(context.Background(), path, time.Now(), int64(len(data)))
		if joined == nil || joined.Address != "newest.example.test:25565" || attempted == nil || attempted.Address != "pending.example.test:25565" || attempted.Confirmed || read != int64(len(data)) {
			t.Fatalf("compressed=%v: joined=%+v attempted=%+v read=%d", compressed, joined, attempted, read)
		}
		// Without a later real join the oversized line must not replace history.
		if !compressed {
			writeTestFile(t, path, data[:bytes.Index(data, []byte(join("newest.example.test")))])
			joined, _, _ := readServerLog(context.Background(), path, time.Now(), int64(len(data)))
			if joined == nil || joined.Address != "older.example.test:25565" {
				t.Fatalf("oversized line produced a false join: %+v", joined)
			}
		}
	}
}

func TestRecentServerStreamRejectsIncompleteOrCorruptArchives(t *testing.T) {
	data := []byte("[20:00:00] [Render thread/INFO]: Connecting to old.example.test, 25565\n" +
		"[20:00:01] [Render thread/INFO]: Loaded 3 advancements\n" + strings.Repeat("ordinary output\n", 4096))
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write(data)
	_ = w.Close()
	corrupt := append([]byte(nil), gz.Bytes()...)
	corrupt[len(corrupt)-8] ^= 0xff
	for _, tc := range []struct {
		name  string
		data  []byte
		limit int64
	}{
		{"truncated", gz.Bytes()[:gz.Len()-4], int64(len(data))},
		{"checksum", corrupt, int64(len(data))},
		{"budget", gz.Bytes(), 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "latest.log.gz")
			writeTestFile(t, path, tc.data)
			joined, attempted, read := readServerLog(context.Background(), path, time.Now(), tc.limit)
			if joined != nil || attempted != nil || read > tc.limit+1 {
				t.Fatalf("invalid archive returned history: joined=%+v attempted=%+v read=%d", joined, attempted, read)
			}
		})
	}
}

func TestRecentServerStreamCancelsAndDropsTailFragments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest.log")
	data := []byte("[20:00:00] [Render thread/INFO]: Connecting to partial.example.test, 25565")
	writeTestFile(t, path, data)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if joined, attempted, read := readServerLog(ctx, path, time.Now(), 1024); joined != nil || attempted != nil || read != 0 {
		t.Fatal("cancelled scan read a log")
	}
	if joined, attempted, _ := readServerLog(context.Background(), path, time.Now(), int64(len(data)-1)); joined != nil || attempted != nil {
		t.Fatal("tail fragment without a newline was parsed as a complete message")
	}
}

func TestRecentServerIgnoresUnrelatedLegacyTextWithoutAllocating(t *testing.T) {
	line := append([]byte("[20:00:00] [Render thread/INFO]: ordinary mod output "), bytes.Repeat([]byte{0xb2, 0xe2, 0xca, 0xd4}, 1024)...)
	state := serverLogState{pending: &InstanceServer{Address: "pending.example.test:25565"}}
	if allocations := testing.AllocsPerRun(100, func() { state.line(line) }); allocations != 0 {
		t.Fatalf("unrelated legacy output allocated %g times", allocations)
	}
}
