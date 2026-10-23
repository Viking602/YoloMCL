package launcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

type testStatusResolver struct{ target *net.TCPAddr }

func (r testStatusResolver) LookupSRV(context.Context, string, string, string) (string, []*net.SRV, error) {
	return "", []*net.SRV{{Target: r.target.IP.String() + ".", Port: uint16(r.target.Port)}}, nil
}

func statusResponse(t *testing.T, reply any) []byte {
	t.Helper()
	b, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	return append(appendServerVarInt([]byte{0}, uint32(len(b))), b...)
}

func TestQueryServerStatusWaitsForStatusBeforePing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var icon bytes.Buffer
	if err := png.Encode(&icon, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	reply := statusResponse(t, map[string]any{
		"description": map[string]any{"text": "§aHello ", "extra": []any{map[string]any{"text": "world"}, "\nWelcome back"}},
		"version":     map[string]any{"name": "NeoForge 1.21.1", "protocol": 767},
		"players":     map[string]any{"online": 8, "max": 20},
		"favicon":     "data:image/png;base64," + base64.StdEncoding.EncodeToString(icon.Bytes()),
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		r := bufio.NewReader(conn)
		b, err := readServerPacket(r)
		if err != nil {
			t.Error(err)
			return
		}
		handshake := bytes.NewReader(b)
		id, _ := readServerVarInt(handshake)
		protocol, _ := readServerVarInt(handshake)
		n, _ := readServerVarInt(handshake)
		host := make([]byte, n)
		_, _ = io.ReadFull(handshake, host)
		tail, _ := io.ReadAll(handshake)
		port := 25565
		if id != 0 || protocol != ^uint32(0) || string(host) != "play.example.test" || !bytes.Equal(tail, []byte{byte(port >> 8), byte(port), 1}) {
			t.Errorf("wrong status handshake: %x", b)
		}
		request, err := readServerPacket(r)
		if err != nil || !bytes.Equal(request, []byte{0}) {
			t.Errorf("wrong status request: %x, %v", request, err)
			return
		}
		// Velocity answers an early ping and closes before its async MOTD. A
		// real status query must wait for this response before sending the ping.
		_ = conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
		if _, err := r.Peek(1); err == nil {
			t.Error("ping sent before the status response")
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		_, _ = conn.Write(serverPacket(reply))
		ping, err := readServerPacket(r)
		if err != nil || len(ping) != 9 || ping[0] != 1 {
			t.Errorf("missing ping after status: %x, %v", ping, err)
			return
		}
		_, _ = conn.Write(serverPacket(ping))
	}()
	st, err := queryServerStatus(context.Background(), "play.example.test", testStatusResolver{listener.Addr().(*net.TCPAddr)})
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if st.Description != "Hello world\nWelcome back" || st.Version != "NeoForge 1.21.1" || !st.PlayersKnown || st.Online != 8 || st.Max != 20 || st.Latency < 0 || st.Latency > time.Second || !bytes.Equal(st.IconPNG, icon.Bytes()) {
		t.Fatalf("wrong public server information: %+v", st)
	}
}

func TestServerStatusRetriesServersThatWaitForPing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	reply := statusResponse(t, map[string]any{"description": "Deferred status"})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for attempt := 0; attempt < 2; attempt++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			reader := bufio.NewReader(conn)
			_, _ = readServerPacket(reader) // handshake
			_, _ = readServerPacket(reader) // status request
			ping, err := readServerPacket(reader)
			if attempt == 0 {
				if err == nil {
					t.Error("first attempt sent an eager ping")
				}
			} else if err != nil || len(ping) != 9 || ping[0] != 1 {
				t.Error("fallback omitted its compatibility ping")
			} else {
				// A proxy can reply to the ping before completing its status.
				_, _ = conn.Write(append(serverPacket(ping), serverPacket(reply)...))
			}
			_ = conn.Close()
		}
	}()
	st, err := QueryServerStatus(context.Background(), listener.Addr().String())
	_ = listener.Close()
	<-done
	if err != nil || st.Description != "Deferred status" {
		t.Fatalf("deferred status compatibility failed: %+v %v", st, err)
	}
}

func TestLiveServerStatusQA(t *testing.T) {
	address := os.Getenv("MCL_SERVER_STATUS_QA")
	if address == "" {
		t.Skip("live server QA not requested")
	}
	st, err := QueryServerStatus(context.Background(), address)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %s, %s, %d/%d players, %v", address, st.Description, st.Version, st.Online, st.Max, st.Latency)
}

func TestServerStatusCancellationClosesStalledConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted, closed := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(closed)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		close(accepted)
		_, _ = io.Copy(io.Discard, conn)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := QueryServerStatus(ctx, listener.Addr().String()); result <- err }()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("connection never opened")
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled request succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt network read")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancelled socket remained open")
	}
}

func TestServerStatusRejectsOversizedAndMalformedPackets(t *testing.T) {
	for _, b := range [][]byte{
		{0}, {255, 255, 255, 255, 255, 0}, appendServerVarInt(nil, maxServerPacket+1), {8, 0, 2},
	} {
		if _, err := readServerPacket(bufio.NewReader(bytes.NewReader(b))); err == nil {
			t.Errorf("accepted bad frame %x", b)
		}
	}
	for _, b := range [][]byte{
		{1, 0}, {0, 6, '{', '}'}, statusResponse(t, map[string]any{}), {0, 1, '!'},
	} {
		if _, err := parseServerStatus(b); err == nil {
			t.Errorf("accepted bad status %x", b)
		}
	}
	if _, err := QueryServerStatus(context.Background(), "127.0.0.1:99999"); err == nil {
		t.Fatal("accepted invalid port")
	}
}

func TestServerDescriptionAndOptionalFields(t *testing.T) {
	st, err := parseServerStatus(statusResponse(t, map[string]any{
		"description": []any{"§bWelcome", map[string]any{"text": "\x00\u202e friends", "extra": []any{"!"}}},
		"favicon":     "data:image/png;base64," + strings.Repeat("a", 97<<10),
	}))
	if err != nil || st.Description != "Welcome friends!" || st.PlayersKnown || len(st.IconPNG) != 0 {
		t.Fatalf("bad optional fields: %+v, %v", st, err)
	}
	if got := serverDescription(strings.Repeat("世", 1000)); len([]rune(got)) != 512 {
		t.Fatal("unbounded MOTD")
	}
	var icon bytes.Buffer
	_ = png.Encode(&icon, image.NewRGBA(image.Rect(0, 0, 256, 256)))
	st, err = parseServerStatus(statusResponse(t, map[string]any{"description": "Online", "favicon": "data:image/png;base64," + base64.StdEncoding.EncodeToString(icon.Bytes())}))
	if err != nil || len(st.IconPNG) != 0 {
		t.Fatal("accepted non-server-sized icon")
	}
}
