package nativeui

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestRecentServerOnlineAndUnreachableCards(t *testing.T) {
	for _, width := range []int{900, 1536} {
		for _, lang := range []string{LangZH, LangEN} {
			t.Run(fmt.Sprintf("%d-%s", width, lang), func(t *testing.T) {
				a, _ := homeTester(t, lang)
				a.contentSwaps = nil
				defer func() { a.cancel(); a.bgWG.Wait() }()
				previousMessage, previousError := a.message, a.errorMessage
				address := "friends.example.test:25565"
				st := launcher.InstanceStatus{Dir: a.root().Path, RecentServer: &launcher.InstanceServer{Address: address, Confirmed: true, Connected: time.Now()}}
				checked := time.Now()
				a.recentPing.cache = map[string]serverProbe{address: {checked: checked, status: &launcher.ServerStatus{
					Description: "Welcome to our world\nBuild, explore and play together! " + strings.Repeat("Very long message ", 12),
					Version:     "NeoForge 1.21.1", PlayersKnown: true, Online: 8, Max: 20, Latency: 42 * time.Millisecond,
				}}}
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					s := uiScale(c)
					ui.Box(c).Fill().Background(bg).Padding(20 * s).Children(func() {
						ui.Row(c).Width(360 * s).Height(260 * s).Children(func() { a.ovServer(c, st) })
					})
				}, width, 1024)
				tt.Advance(400 * time.Millisecond)
				if !tt.HasText("42 ms") || !tt.HasText("NeoForge 1.21.1") || !tt.HasText("8 / 20") || !tt.HasText(a.L("在线玩家", "Players online")) {
					t.Fatalf("missing server information: %v", tt.Texts())
				}
				card, _ := tt.Find(a.L("最近服务器", "Recent server"))
				info, _ := tt.Find("服务器信息")
				copy, _ := tt.Find("按钮文字 ov-copy-server")
				if info.Y+info.H > copy.Y || copy.Y+copy.H > card.Y+card.H {
					t.Fatalf("card content overlaps: %+v %+v %+v", card, info, copy)
				}
				clickUI(t, tt, a.L("复制地址", "Copy address"))
				if tt.Clipboard() != strings.TrimSuffix(address, ":25565") {
					t.Fatal("copy did not match the displayed server address")
				}
				a.message, a.errorMessage = previousMessage, previousError
				for i := 0; i < 3; i++ {
					tt.Frame()
				}
				if a.recentPing.loading || a.recentPing.cache[address].checked != checked {
					t.Fatal("cached server was queried again")
				}
				snapshotUI(t, tt, fmt.Sprintf("server-online-%d-%s", width, lang))
				a.recentPing.cache[address] = serverProbe{checked: checked}
				tt.Frame()
				tt.Advance(400 * time.Millisecond)
				if !tt.HasText(a.L("服务器不可达", "Server unreachable")) || tt.HasText("42 ms") || tt.HasText("NeoForge 1.21.1") || a.message != previousMessage || a.errorMessage != previousError {
					t.Fatalf("unreachable card retained stale data or raised an error: %v error=%v %s", tt.Texts(), a.errorMessage, a.message)
				}
				snapshotUI(t, tt, fmt.Sprintf("server-offline-%d-%s", width, lang))
			})
		}
	}
}

func TestRecentServerRefreshComparesValues(t *testing.T) {
	a, old := homeTester(t, LangEN)
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.bgWG.Wait()
	old.Frame()
	a.contentSwaps = nil
	a.overviewHolding = true // this test never starts a network probe
	st := launcher.InstanceStatus{Dir: a.root().Path, RecentServer: &launcher.InstanceServer{Address: "old.example.test:25565"}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Row(c).Width(360).Height(260).Children(func() { a.ovServer(c, st) })
	}, 400, 300)
	for range 3 {
		server := *st.RecentServer
		st.RecentServer = &server
		tt.Frame()
		if state := a.contentSwaps["ov-server"]; state.leaving || state.serial != 0 {
			t.Fatal("identical server data from a new publication triggered an animation")
		}
	}
	// A real field change must still animate, even if the pointer is reused.
	st.RecentServer.Address = "new.example.test:25565"
	tt.Frame()
	if !a.contentSwaps["ov-server"].leaving {
		t.Fatal("changed server fields did not trigger a transition")
	}
	tt.Advance(400 * time.Millisecond)
	if state := a.contentSwaps["ov-server"]; state.leaving || state.serial != 1 || !tt.HasText("new.example.test") {
		t.Fatal("server transition failed to settle on the new value")
	}
}

func TestRecentServerSwitchCancelsPendingQuery(t *testing.T) {
	a, tt := homeTester(t, LangZH)
	defer func() { a.cancel(); a.bgWG.Wait() }()
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
	a.statusGen++
	a.instStatusKey = a.root().ID + "|" + a.root().SelectedVersion
	a.instStatus = launcher.InstanceStatus{Dir: a.root().Path, RecentServer: &launcher.InstanceServer{Address: listener.Addr().String(), Connected: time.Now()}}
	tt.Frame()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	if !a.recentPing.loading {
		t.Fatal("missing pending probe")
	}
	a.applySelection("r1", "1.21.8")
	a.loadInstanceStatus()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("switch left the old probe running")
	}
	tt.Frame()
	if a.recentPing.loading || len(a.recentPing.cache) != 0 || tt.HasText("服务器不可达") {
		t.Fatal("old probe leaked into new instance")
	}
}

func TestRecentServerExpiredCacheRefreshesSilently(t *testing.T) {
	a, tt := homeTester(t, LangZH)
	defer func() { a.cancel(); a.bgWG.Wait() }()
	previousMessage, previousError := a.message, a.errorMessage
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	a.statusGen++
	a.instStatusKey = a.root().ID + "|" + a.root().SelectedVersion
	a.instStatus = launcher.InstanceStatus{Dir: a.root().Path, RecentServer: &launcher.InstanceServer{Address: address, Connected: time.Now()}}
	a.recentPing.cache = map[string]serverProbe{address: {checked: time.Now().Add(-time.Minute), status: &launcher.ServerStatus{Version: "Old version"}}}
	waitUI(t, tt, func() bool { return !a.recentPing.loading && a.recentPing.cache[address].status == nil })
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("服务器不可达") || a.message != previousMessage || a.errorMessage != previousError || tt.HasText("Old version") {
		t.Fatalf("unreachable result did not replace stale status quietly: %v error=%v %s", tt.Texts(), a.errorMessage, a.message)
	}
}
