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
	if _, err = readRecentServer(ctx, r.Path); !errors.Is(err, context.Canceled) {
		t.Fatalf("obsolete server scan did not stop: %v", err)
	}
}

func TestRecentServerReadsAreBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest.log")
	tail := "[22:00:00] [Render thread/INFO]: Connecting to tail.example.test, 25565\n[22:00:01] [Render thread/INFO]: Loaded 3 advancements\n"
	writeTestFile(t, path, []byte(strings.Repeat("old log data\n", 10000)+tail))
	b, _ := readServerLog(context.Background(), path, 1024)
	if len(b) > 1024 || !bytes.HasSuffix(b, []byte(tail)) {
		t.Fatal("large plain log was not read from its bounded tail")
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(strings.Repeat("log\n", 10000)))
	_ = w.Close()
	writeTestFile(t, path+".gz", gz.Bytes())
	if b, read := readServerLog(context.Background(), path+".gz", 1024); b != nil || read <= 1024 {
		t.Fatal("truncated archive could misidentify an older connection as latest")
	}
}
