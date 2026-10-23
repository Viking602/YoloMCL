package launcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ServerStatus is a Java server's public server-list response, not a login.
type ServerStatus struct {
	Description, Version string
	Online, Max          int
	PlayersKnown         bool
	Latency              time.Duration
	IconPNG              []byte
}

const maxServerPacket = 1 << 20

type serverStatusResolver interface {
	LookupSRV(context.Context, string, string, string) (string, []*net.SRV, error)
}

// QueryServerStatus uses Java's Server List Ping protocol directly. The whole
// request, including DNS, is bounded and cancellation closes the socket.
// https://minecraft.wiki/w/Java_Edition_protocol/Server_List_Ping
func QueryServerStatus(ctx context.Context, address string) (ServerStatus, error) {
	return queryServerStatus(ctx, address, net.DefaultResolver)
}

func queryServerStatus(ctx context.Context, address string, resolver serverStatusResolver) (ServerStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host, port = strings.Trim(address, "[]"), "25565"
	}
	if serverAddress(host, port) == "" {
		return ServerStatus{}, errors.New("invalid server address")
	}
	// SRV changes the TCP destination, not the virtual host in the handshake.
	// Proxies use the original hostname and port to select a backend.
	dialHost, dialPort := host, port
	if port == "25565" && net.ParseIP(host) == nil {
		dnsCtx, stop := context.WithTimeout(ctx, 700*time.Millisecond)
		_, records, e := resolver.LookupSRV(dnsCtx, "minecraft", "tcp", host)
		stop()
		if e == nil && len(records) > 0 {
			dialHost = strings.TrimSuffix(records[0].Target, ".")
			dialPort = strconv.Itoa(int(records[0].Port))
			if serverAddress(dialHost, dialPort) == "" {
				return ServerStatus{}, errors.New("invalid server SRV record")
			}
		}
	}
	target := net.JoinHostPort(dialHost, dialPort)
	status, err := requestServerStatus(ctx, target, host, port, false)
	// A few servers delay their status until a ping arrives. Only retry those
	// on a fresh connection: an eager ping makes Velocity send a pong and close
	// before its asynchronous status response is ready.
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() && ctx.Err() == nil {
		return requestServerStatus(ctx, target, host, port, true)
	}
	return status, err
}

func requestServerStatus(ctx context.Context, target, host, port string, earlyPing bool) (ServerStatus, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", target)
	if err != nil {
		return ServerStatus{}, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if !earlyPing {
		deadline = minServerDeadline(deadline, time.Now().Add(1400*time.Millisecond))
	}
	_ = conn.SetDeadline(deadline)
	return exchangeServerStatus(conn, host, port, earlyPing)
}

func minServerDeadline(a, b time.Time) time.Time {
	if a.IsZero() || b.Before(a) {
		return b
	}
	return a
}

func exchangeServerStatus(conn net.Conn, host, port string, earlyPing bool) (ServerStatus, error) {
	p, _ := strconv.Atoi(port)
	handshake := []byte{0} // handshake, protocol -1, address, port, status state
	handshake = appendServerVarInt(handshake, ^uint32(0))
	handshake = appendServerVarInt(handshake, uint32(len(host)))
	handshake = append(handshake, host...)
	handshake = append(handshake, byte(p>>8), byte(p), 1)
	request := serverPacket(handshake)
	request = append(request, 1, 0) // empty status request
	ping := make([]byte, 9)
	ping[0] = 1
	binary.BigEndian.PutUint64(ping[1:], uint64(time.Now().UnixNano()))
	if earlyPing {
		request = append(request, serverPacket(ping)...)
	}
	started := time.Now()
	if _, err := io.Copy(conn, bytes.NewReader(request)); err != nil {
		return ServerStatus{}, err
	}
	reader := bufio.NewReader(conn)
	var status ServerStatus
	var err error
	pongReceived := false
	for packets := 0; ; packets++ {
		packet, readErr := readServerPacket(reader)
		if readErr != nil {
			return ServerStatus{}, readErr
		}
		if earlyPing && !pongReceived && bytes.Equal(packet, ping) {
			pongReceived = true
			continue
		}
		if packets > 1 {
			return ServerStatus{}, errors.New("unexpected server status packets")
		}
		status, err = parseServerStatus(packet)
		if err != nil {
			return ServerStatus{}, err
		}
		break
	}
	status.Latency = time.Since(started)
	if pongReceived {
		return status, nil
	}
	// Once status arrived, a missing optional pong must not hide valid info or
	// consume the full query timeout. Cancellation still closes this socket.
	_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
	if !earlyPing {
		if _, err := io.Copy(conn, bytes.NewReader(serverPacket(ping))); err != nil {
			return status, nil
		}
	}
	if pong, e := readServerPacket(reader); e == nil && bytes.Equal(pong, ping) {
		status.Latency = time.Since(started)
	}
	return status, nil
}

func serverPacket(payload []byte) []byte {
	return append(appendServerVarInt(nil, uint32(len(payload))), payload...)
}

func appendServerVarInt(dst []byte, n uint32) []byte {
	for n >= 128 {
		dst = append(dst, byte(n)|128)
		n >>= 7
	}
	return append(dst, byte(n))
}

func readServerVarInt(r io.ByteReader) (uint32, error) {
	var n uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if i == 4 && b > 15 {
			return 0, errors.New("invalid server VarInt")
		}
		n |= uint32(b&127) << (7 * i)
		if b < 128 {
			return n, nil
		}
	}
	return 0, errors.New("invalid server VarInt")
}

func readServerPacket(r *bufio.Reader) ([]byte, error) {
	n, err := readServerVarInt(r)
	if err != nil {
		return nil, err
	}
	if n == 0 || n > maxServerPacket {
		return nil, errors.New("invalid server packet length")
	}
	b := make([]byte, int(n))
	_, err = io.ReadFull(r, b)
	return b, err
}

func parseServerStatus(packet []byte) (ServerStatus, error) {
	r := bytes.NewReader(packet)
	id, err := readServerVarInt(r)
	if err != nil || id != 0 {
		return ServerStatus{}, errors.New("invalid server status packet")
	}
	n, err := readServerVarInt(r)
	if err != nil || n == 0 || n != uint32(r.Len()) {
		return ServerStatus{}, errors.New("invalid server status length")
	}
	var reply struct {
		Description any `json:"description"`
		Version     *struct {
			Name string `json:"name"`
		} `json:"version"`
		Players *struct {
			Online int `json:"online"`
			Max    int `json:"max"`
		} `json:"players"`
		Favicon string `json:"favicon"`
	}
	if err = json.NewDecoder(r).Decode(&reply); err != nil {
		return ServerStatus{}, err
	}
	if reply.Description == nil && reply.Version == nil && reply.Players == nil {
		return ServerStatus{}, errors.New("empty server status")
	}
	st := ServerStatus{Description: serverDescription(reply.Description)}
	if reply.Version != nil {
		st.Version = cleanServerText(reply.Version.Name, 128)
	}
	if reply.Players != nil && reply.Players.Online >= 0 && reply.Players.Max >= 0 {
		st.Online, st.Max, st.PlayersKnown = reply.Players.Online, reply.Players.Max, true
	}
	if raw, ok := strings.CutPrefix(reply.Favicon, "data:image/png;base64,"); ok && len(raw) <= 96<<10 {
		if b, e := base64.StdEncoding.DecodeString(raw); e == nil {
			// A server icon is 64x64. Validate before the UI decodes any pixels.
			if cfg, e := png.DecodeConfig(bytes.NewReader(b)); e == nil && cfg.Width == 64 && cfg.Height == 64 {
				st.IconPNG = b
			}
		}
	}
	return st, nil
}

func serverDescription(value any) string {
	var out strings.Builder
	var walk func(any, int)
	walk = func(v any, depth int) {
		if depth > 32 || out.Len() >= 8192 {
			return
		}
		switch v := v.(type) {
		case string:
			out.WriteString(v[:min(len(v), 8192-out.Len())])
		case []any:
			for _, part := range v {
				walk(part, depth+1)
			}
		case map[string]any:
			if text, ok := v["text"].(string); ok {
				walk(text, depth+1)
			} else if fallback, ok := v["fallback"].(string); ok {
				walk(fallback, depth+1)
			} else if key, ok := v["translate"].(string); ok {
				walk(key, depth+1)
			}
			walk(v["extra"], depth+1)
		}
	}
	walk(value, 0)
	return cleanServerText(out.String(), 512)
}

func cleanServerText(s string, limit int) string {
	s = strings.ToValidUTF8(stripFormatting(s), "")
	out := make([]rune, 0, min(len(s), limit))
	for _, r := range s {
		if r == '\r' || (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.In(r, unicode.Cf) {
			continue
		}
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return strings.TrimSpace(string(out))
}
