package launcher

import (
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

func serverFromLog(ctx context.Context, data []byte, file string, modified time.Time) (joined, attempted *InstanceServer) {
	var pending *InstanceServer
	for _, raw := range bytes.Split(data, []byte{'\n'}) {
		if ctx.Err() != nil {
			return nil, nil
		}
		line := strings.TrimSpace(DecodeLine(raw))
		i := strings.LastIndex(line, "]: ")
		if i < 0 {
			continue
		}
		msg := line[i+3:]
		if m := serverConnectRE.FindStringSubmatch(msg); m != nil && strings.Contains(line[:i], "/INFO") {
			if address := serverAddress(m[1], m[2]); address != "" {
				pending = &InstanceServer{Address: address, Connected: serverLogTime(line, file, modified)}
				attempted = pending
			}
			continue
		}
		if pending == nil {
			continue
		}
		if serverJoinedRE.MatchString(msg) || msg == "Connected to a modded server." {
			pending.Confirmed = true
			joined, pending = pending, nil
		} else if strings.HasPrefix(msg, "Couldn't connect to server") || strings.HasPrefix(msg, "Failed to connect") || strings.HasPrefix(msg, "Disconnecting") || strings.HasPrefix(msg, "Starting integrated minecraft server") {
			pending = nil
		}
	}
	return joined, attempted
}

// Bounded, cancellable reads keep large debug logs and archive collections from
// delaying a cold instance switch. No server is contacted or inferred from the
// order of saved favourites. Debug logs duplicate latest.log and are excluded.
func readRecentServer(ctx context.Context, dir string) (*InstanceServer, error) {
	logs, err := safeJoin(dir, "logs")
	if err != nil {
		return nil, nil
	}
	entries, _ := os.ReadDir(logs)
	var files []os.FileInfo
	for _, en := range entries {
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
		data, read := readServerLog(ctx, filepath.Join(dir, "logs", fi.Name()), limit)
		budget -= read
		joined, pending := serverFromLog(ctx, data, fi.Name(), fi.ModTime())
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

func readServerLog(ctx context.Context, path string, limit int64) ([]byte, int64) {
	f, e := os.Open(path)
	if e != nil {
		return nil, 0
	}
	defer f.Close()
	var reader io.Reader = f
	tail := false
	compressed := strings.HasSuffix(strings.ToLower(path), ".gz")
	if compressed {
		gz, e := gzip.NewReader(serverContextReader{ctx, f})
		if e != nil {
			return nil, 0
		}
		defer gz.Close()
		reader = gz
	} else if fi, e := f.Stat(); e == nil && fi.Size() > limit {
		_, _ = f.Seek(-limit, io.SeekEnd)
		tail = true
	}
	data, e := io.ReadAll(io.LimitReader(serverContextReader{ctx, reader}, limit+1))
	read := int64(len(data))
	if e != nil || (compressed && int64(len(data)) > limit) {
		return nil, read // an incomplete archive may hide a newer connection
	}
	if tail {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	return data, read
}
