package launcher

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// InstanceServer is the most recent multiplayer connection found in this
// instance's own logs. Confirmed distinguishes a join from a connection attempt.
type InstanceServer struct {
	Address   string    `json:"address"`
	Connected time.Time `json:"connected"`
	Confirmed bool      `json:"confirmed"`
}

var (
	serverConnectRE = regexp.MustCompile(`^Connecting to (.+), ([0-9]+)$`)
	serverJoinedRE  = regexp.MustCompile(`^Loaded [0-9]+ advancements$`)
	serverTimeRE    = regexp.MustCompile(`\b([0-2][0-9]:[0-5][0-9]:[0-5][0-9])(?:\.[0-9]+)?\b`)
)

func serverLogTime(line, file string, modified time.Time) time.Time {
	day := modified.In(time.Local)
	if len(file) >= 10 {
		if date, e := time.ParseInLocation("2006-01-02", file[:10], time.Local); e == nil {
			day = date
		}
	}
	if clock := serverTimeRE.FindStringSubmatch(line); clock != nil {
		if at, e := time.ParseInLocation("2006-01-02 15:04:05", day.Format("2006-01-02")+" "+clock[1], time.Local); e == nil {
			if at.After(modified.Add(time.Minute)) {
				at = at.AddDate(0, 0, -1) // a latest.log spanning midnight
			}
			return at
		}
	}
	return modified
}

func serverAddress(host, port string) string {
	n, e := strconv.Atoi(port)
	if e != nil || n < 1 || n > 65535 {
		return ""
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" || len(host) > 253 || strings.ContainsAny(host, " /\\\t\r\n\x00") {
		return ""
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(n))
}

type serverLogState struct {
	file                       string
	modified                   time.Time
	joined, attempted, pending *InstanceServer
}

func (s *serverLogState) line(raw []byte) {
	raw = bytes.TrimSpace(raw)
	i := bytes.LastIndex(raw, []byte("]: "))
	if i < 0 {
		return
	}
	msg := raw[i+3:]
	// Connection markers are ASCII in both UTF-8 and legacy Windows logs.
	// Only decode a candidate connection; ordinary mod output needs neither
	// a string allocation nor charset detection during overview navigation.
	if bytes.HasPrefix(msg, []byte("Connecting to ")) && bytes.Contains(raw[:i], []byte("/INFO")) {
		line := DecodeLine(raw)
		if m := serverConnectRE.FindStringSubmatch(line[strings.LastIndex(line, "]: ")+3:]); m != nil {
			if address := serverAddress(m[1], m[2]); address != "" {
				s.pending = &InstanceServer{Address: address, Connected: serverLogTime(line, s.file, s.modified)}
				s.attempted = s.pending
			}
		}
		return
	}
	if s.pending == nil {
		return
	}
	if serverJoinedRE.Match(msg) || bytes.Equal(msg, []byte("Connected to a modded server.")) {
		s.pending.Confirmed = true
		s.joined, s.pending = s.pending, nil
	} else if bytes.HasPrefix(msg, []byte("Couldn't connect to server")) || bytes.HasPrefix(msg, []byte("Failed to connect")) || bytes.HasPrefix(msg, []byte("Disconnecting")) || bytes.HasPrefix(msg, []byte("Starting integrated minecraft server")) {
		s.pending = nil
	}
}

// Bounded, cancellable reads keep large debug logs and archive collections from
// delaying a cold instance switch. No server is contacted or inferred from the
// order of saved favourites. Debug logs duplicate latest.log and are excluded.
func readRecentServer(ctx context.Context, dir string, entries []os.DirEntry) (*InstanceServer, error) {
	var files []os.FileInfo
	for _, en := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.ToLower(en.Name())
		if !en.Type().IsRegular() || strings.HasPrefix(name, "debug") || (!strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".log.gz")) {
			continue
		}
		if fi, e := en.Info(); e == nil {
			files = append(files, fi)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime().After(files[j].ModTime()) })
	budget := int64(16 << 20)
	var attempt *InstanceServer
	for _, fi := range files[:min(12, len(files))] {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if budget <= 0 {
			break
		}
		limit := min(budget, 8<<20)
		joined, pending, read := readServerLog(ctx, filepath.Join(dir, "logs", fi.Name()), fi.ModTime(), limit)
		budget -= read
		if joined != nil {
			return joined, ctx.Err()
		}
		if pending != nil && (attempt == nil || pending.Connected.After(attempt.Connected)) {
			attempt = pending
		}
	}
	return attempt, ctx.Err()
}

type serverContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r serverContextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}

// A fixed line buffer also bounds pathological single-line mod dumps. Such a
// line is skipped in its entirety, never interpreted as several log messages.
const serverLogLineSize = 32 << 10

func readServerLog(ctx context.Context, path string, modified time.Time, limit int64) (joined, attempted *InstanceServer, read int64) {
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, 0
	}
	defer f.Close()
	var reader io.Reader = f
	tail := false
	compressed := strings.HasSuffix(strings.ToLower(path), ".gz")
	if compressed {
		gz, e := gzip.NewReader(serverContextReader{ctx, f})
		if e != nil {
			return nil, nil, 0
		}
		defer gz.Close()
		reader = gz
	} else if fi, e := f.Stat(); e == nil && fi.Size() > limit {
		if _, e := f.Seek(-limit, io.SeekEnd); e != nil {
			return nil, nil, 0
		}
		tail = true
	}
	limited := &io.LimitedReader{R: serverContextReader{ctx, reader}, N: limit + 1}
	lines := bufio.NewReaderSize(limited, serverLogLineSize)
	state := serverLogState{file: filepath.Base(path), modified: modified}
	discard := tail
	for {
		if ctx.Err() != nil {
			return nil, nil, limit + 1 - limited.N
		}
		line, err := lines.ReadSlice('\n')
		read = limit + 1 - limited.N
		if err == bufio.ErrBufferFull {
			discard = true
			continue
		}
		if err != nil && err != io.EOF || compressed && read > limit {
			return nil, nil, read // a corrupt/incomplete archive may hide a newer connection
		}
		if !discard {
			state.line(line)
		}
		discard = false
		if err == io.EOF {
			return state.joined, state.attempted, read
		}
	}
}
