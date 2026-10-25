package nativeui

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func accountMenuTester(t *testing.T, lang string, width int) (*App, *ui.Tester, *int) {
	t.Helper()
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	a.OverrideLanguage(lang)
	a.accountTab = a.tr().AccTabMicrosoft
	a.snapshot.Services.Microsoft = true
	a.snapshot.Accounts = []launcher.Account{
		{ID: "main", Name: "SnowiceLemon", Kind: "microsoft", UUID: acctFixture[0].UUID},
		{ID: "offline", Name: "Steve", Kind: "offline", UUID: acctFixture[1].UUID},
	}
	a.snapshot.ActiveAccount = "main"
	a.avatars.heads = map[string]*ui.Bitmap{}
	for _, ac := range a.snapshot.Accounts {
		a.avatars.heads[ac.ID] = ui.NewBitmap(defaultHeadFor(ac)).Pixelated()
	}
	behind := new(int)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		root := ui.Column(c).Fill().Background(chromeBg)
		root.Children(func() {
			ui.Row(c).FillWidth().Height(48).PaddingX(18).Justify(ui.End).AlignItems(ui.Center).Children(func() { a.accountEntry(c) })
			if ui.ButtonBase(c).Label("菜单后的内容").FillWidth().Height(672).Background(bg).Children(func() {}).Clicked() {
				*behind++
			}
		})
		a.dialogView(c, root)
	}, width, 720)
	return a, tt, behind
}

// Exercise the rendered destinations, not just the state set by the menu.
func TestAccountMenuDistinctDialogs(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		for _, width := range []int{900, 1536} {
			t.Run(fmt.Sprintf("%s-%d", lang, width), func(t *testing.T) {
				a, tt, _ := accountMenuTester(t, lang, width)
				if width == 1536 {
					tt.SetSize(width, 1024)
				}
				assertManage := func() {
					t.Helper()
					tt.Advance(300 * time.Millisecond)
					if _, ok := tt.Find("弹窗标题 " + a.tr().DialogTitleAccount); !ok || !tt.HasText(a.tr().AccSavedSection) {
						t.Fatal("Manage must show the saved account list")
					}
					for _, ac := range a.snapshot.Accounts {
						if _, ok := tt.Find("账号行 " + ac.ID); !ok {
							t.Fatalf("saved account %s missing", ac.ID)
						}
					}
					if _, ok := tt.Find("账号登录方式"); ok || tt.HasText(a.tr().AccMSDesc) {
						t.Fatal("Manage unexpectedly shows the add form")
					}
				}
				assertAdd := func() {
					t.Helper()
					tt.Advance(300 * time.Millisecond)
					if _, ok := tt.Find("弹窗标题 " + a.tr().AccAddAccountBtn); !ok {
						t.Fatal("Add must have its own title instead of Manage Accounts")
					}
					for _, text := range []string{a.tr().AccTabMicrosoft, a.tr().AccTabOffline, a.tr().AccTabThirdParty} {
						if !tt.HasText(text) {
							t.Fatalf("Add is missing authentication method %q", text)
						}
					}
					if _, ok := tt.Find("账号列表"); ok {
						t.Fatal("Add unexpectedly shows the saved account list")
					}
				}
				openMenu := func() {
					t.Helper()
					clickUI(t, tt, a.tr().SideAccountManage)
					tt.Advance(250 * time.Millisecond)
				}
				closeDialog := func() {
					t.Helper()
					clickUI(t, tt, a.tr().DialogCloseBtn)
					tt.Advance(250 * time.Millisecond)
				}
				openMenu()
				clickUI(t, tt, a.L("账号管理", "Manage accounts"))
				assertManage()
				snapshotUI(t, tt, fmt.Sprintf("account-manage-%s-%d", lang, width))
				closeDialog()
				openMenu()
				clickUI(t, tt, a.tr().AccAddAccountBtn)
				assertAdd()
				snapshotUI(t, tt, fmt.Sprintf("account-add-%s-%d", lang, width))
				// Device polling must follow the Add route after it is split out.
				a.device = &launcher.DeviceInfo{ID: "expired"}
				a.deviceUntil = time.Now().Add(-time.Minute)
				tt.Frame()
				if a.device != nil || !tt.HasText(a.tr().AccMSExpired) {
					t.Fatal("the Add dialog no longer handles Microsoft device expiry")
				}
				clickUI(t, tt, a.tr().AccTabOffline)
				if !tt.HasText(a.tr().AccOfflineDesc) {
					t.Fatal("Add did not switch to the offline form")
				}
				a.form.Username, a.form.Password = "Draft", "Draft password"
				clickUI(t, tt, a.tr().AccBackBtn)
				assertManage()
				if a.form.Username != "" || a.form.Password != "" {
					t.Fatal("returning to Manage retained the abandoned login draft")
				}
				clickUI(t, tt, a.tr().AccAddAccountBtn)
				assertAdd()
				closeDialog()
				openMenu()
				clickUI(t, tt, a.L("账号管理", "Manage accounts"))
				assertManage()
			})
		}
	}
}

func TestAccountMenuWithoutSavedAccounts(t *testing.T) {
	a, tt, _ := accountMenuTester(t, LangZH, 1536)
	a.snapshot.Accounts, a.snapshot.ActiveAccount = nil, ""
	tt.Frame()
	clickUI(t, tt, a.tr().SideAccountManage)
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find("弹窗标题 " + a.tr().AccAddAccountBtn); !ok || !tt.HasText(a.tr().AccMSDesc) {
		t.Fatal("the first account entry must open Add directly")
	}
	a.closeDialog()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	a.openDialog("account")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find("弹窗标题 " + a.tr().DialogTitleAccount); !ok || !tt.HasText("还没有账号") {
		t.Fatal("empty Manage must keep its own title and empty state")
	}
	snapshotUI(t, tt, "account-manage-empty")
	clickUI(t, tt, "按钮文字 account-add-open")
	tt.Advance(300 * time.Millisecond)
	if _, ok := tt.Find("账号登录方式"); !ok {
		t.Fatal("empty Manage cannot open Add")
	}
	if !a.launcherRestartBlocked() {
		t.Fatal("a login draft must block launcher restarts")
	}
	a.closeDialog()
	if a.launcherRestartBlocked() {
		t.Fatal("a closed login draft still blocks launcher restarts")
	}
}

func TestAccountMenuEnterExitAndReopen(t *testing.T) {
	a, tt, behind := accountMenuTester(t, LangZH, 1536)
	clickUI(t, tt, a.tr().SideAccountManage)
	tt.Advance(55 * time.Millisecond)
	mid, ok := tt.Find("账号快速切换")
	if !ok || !a.accountMenuOpen {
		t.Fatal("menu did not open")
	}
	midPixels := overviewRegionPixels(tt.Image(), mid)
	snapshotUI(t, tt, "account-menu-entering")
	tt.Advance(250 * time.Millisecond)
	end, _ := tt.Find("账号快速切换")
	if mid.W >= end.W || mid.H >= end.H || mid.W < .98*end.W || math.Abs(float64(mid.X+mid.W-end.X-end.W)) > .5 || math.Abs(float64(mid.Y-end.Y)) > .5 {
		t.Fatalf("menu did not expand gently from the anchor: mid=%+v end=%+v", mid, end)
	}
	if bytes.Equal(midPixels, overviewRegionPixels(tt.Image(), end)) {
		t.Fatal("menu appeared without fading")
	}
	snapshotUI(t, tt, "account-menu-open")
	add, _ := tt.Find(a.tr().AccAddAccountBtn)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(45 * time.Millisecond)
	if a.accountMenuOpen {
		t.Fatal("Escape did not dismiss the menu")
	}
	closingPixels := overviewRegionPixels(tt.Image(), end)
	snapshotUI(t, tt, "account-menu-closing")
	// Exit copies must be inert: clicking the fading Add row reaches the page.
	tt.ClickAt(add.X+add.W/2, add.Y+add.H/2)
	if a.dialogOpen || *behind != 1 {
		t.Fatal("closing menu retained clickable actions")
	}
	clickUI(t, tt, a.tr().SideAccountManage)
	if !a.accountMenuOpen {
		t.Fatal("clicking during the exit did not reopen the menu")
	}
	reopened, _ := tt.Find("账号快速切换")
	reopenedPixels := overviewRegionPixels(tt.Image(), end)
	difference := 0
	for i, v := range reopenedPixels {
		difference += int(math.Abs(float64(int(v) - int(closingPixels[i]))))
	}
	// Ghost and live glyph edges can round differently by a pixel (allow 1%
	// of the color range). Opacity and scale must continue rather than restart.
	if reopened.W <= .99*end.W || reopened.W >= end.W || difference*100 > len(closingPixels)*255 {
		t.Fatalf("reopening snapped the menu: rect=%+v mean pixel change=%.2f", reopened, float64(difference)/float64(len(closingPixels)))
	}
	snapshotUI(t, tt, "account-menu-reopened")
	tt.Advance(250 * time.Millisecond)
	clickUI(t, tt, a.tr().SideAccountManage)
	tt.Advance(250 * time.Millisecond)
	if a.accountMenuOpen {
		t.Fatal("trigger did not toggle the menu closed")
	}
	if _, ok := tt.Find("账号快速切换"); ok || bytes.Equal(closingPixels, overviewRegionPixels(tt.Image(), end)) {
		t.Fatal("exit did not finish removing the menu")
	}
}

func TestAccountMenuDismissAndActions(t *testing.T) {
	for _, action := range []string{"outside", "active-account", "manage", "add"} {
		t.Run(action, func(t *testing.T) {
			a, tt, behind := accountMenuTester(t, LangZH, 1536)
			clickUI(t, tt, a.tr().SideAccountManage)
			tt.Advance(250 * time.Millisecond)
			switch action {
			case "outside":
				tt.ClickAt(100, 600)
			case "active-account":
				clickUI(t, tt, "切换到 SnowiceLemon")
			case "manage":
				clickUI(t, tt, "账号管理")
			case "add":
				clickUI(t, tt, a.tr().AccAddAccountBtn)
			}
			if a.accountMenuOpen {
				t.Fatal("action did not dismiss the menu")
			}
			wantDialog := action == "manage" || action == "add"
			if a.dialogOpen != wantDialog || (a.dialog == "accountAdd") != (action == "add") {
				t.Fatalf("menu action changed: dialog=%v route=%s", a.dialogOpen, a.dialog)
			}
			if action == "outside" {
				tt.Advance(250 * time.Millisecond)
				before := *behind
				tt.ClickAt(100, 600)
				if *behind != before+1 {
					t.Fatal("dismissed menu left an input blocker over the page")
				}
			}
		})
	}
}

func TestAccountMenuReducedMotionAndLayout(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		for _, width := range []int{900, 1536} {
			t.Run(fmt.Sprintf("%s-%d", lang, width), func(t *testing.T) {
				a, tt, _ := accountMenuTester(t, lang, width)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				clickUI(t, tt, a.tr().SideAccountManage)
				pop, ok := tt.Find("账号快速切换")
				anchor, _ := tt.Find(a.tr().SideAccountManage)
				if !ok || pop.X < 0 || pop.X+pop.W > float32(width) || pop.Y < anchor.Y+anchor.H || math.Abs(float64(pop.X+pop.W-anchor.X-anchor.W)) > .5 {
					t.Fatalf("menu is not anchored within the window: pop=%+v anchor=%+v", pop, anchor)
				}
				pixels := overviewRegionPixels(tt.Image(), pop)
				tt.Advance(250 * time.Millisecond)
				if !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), pop)) {
					t.Fatal("reduced motion still animated the menu")
				}
				snapshotUI(t, tt, fmt.Sprintf("account-menu-%s-%d", lang, width))
				tt.Key(0, ui.KeyEscape)
				if a.accountMenuOpen || bytes.Equal(pixels, overviewRegionPixels(tt.Image(), pop)) {
					t.Fatal("reduced motion did not close the menu immediately")
				}
			})
		}
	}
}

func TestAccountMenuKeepsFollowingContentIdentity(t *testing.T) {
	a, _, _ := accountMenuTester(t, LangZH, 1536)
	var contentID uint64
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).FillWidth().Height(48).Children(func() { a.accountEntry(c) })
			content := ui.Scroll(c).FillWidth().Grow(1)
			contentID = content.ID()
			content.Children(func() { ui.Text(c, "Page content") })
		})
	}, 1536, 1024)
	want := contentID
	clickUI(t, tt, a.tr().SideAccountManage)
	if contentID != want {
		t.Fatalf("opening the account menu recreated the following content: %d -> %d", want, contentID)
	}
	tt.Advance(250 * time.Millisecond)
	tt.Key(0, ui.KeyEscape)
	if contentID != want {
		t.Fatal("closing the account menu recreated the following content")
	}
}

func TestAccountMenuKeepsDiscoverPageStable(t *testing.T) {
	for _, width := range []int{900, 1536} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a, _, _ := accountMenuTester(t, LangZH, width)
			a.page, a.provider, a.resourceType = "discover", "modrinth", "modpack"
			seedDiscoverCategories(a)
			q := launcher.SearchQuery{Source: "modrinth", Type: "modpack"}
			projects := discoverMotionProjects(0, 54)
			a.acceptDiscoverResults(q, launcher.SearchResult{Projects: projects, Total: len(projects), Limit: len(projects)})
			tt := ui.NewTester(a.View, width, 1024)
			// Native caption buttons leave the account entry last in the bar.
			// The default headless custom buttons previously hid the key leak.
			tt.SetTitleBar(ui.TitleBar{Right: 138})
			tt.Advance(time.Second)
			tt.Scroll(float32(width)-100, 700, 0, 480)
			tt.Advance(time.Second)
			page, ok := tt.Find("发现视图")
			if !ok || a.discoverMotion.scroll.st.Y <= 0 {
				t.Fatal("discovery page did not load and scroll")
			}
			scrollY := a.discoverMotion.scroll.st.Y
			// Stay below the account menu, its shadow and the title tooltip.
			region := ui.Rect{X: page.X + 20, Y: 350, W: page.W - 40, H: 500}
			pixels := overviewRegionPixels(tt.Image(), region)
			assertStable := func(stage string) {
				t.Helper()
				if got, ok := tt.Find("发现视图"); !ok || got != page {
					t.Fatalf("%s moved the page: before=%+v after=%+v", stage, page, got)
				}
				if a.discoverMotion.scroll.st.Y != scrollY {
					t.Fatalf("%s reset the scroll position: %v -> %v", stage, scrollY, a.discoverMotion.scroll.st.Y)
				}
				if !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), region)) {
					snapshotUI(t, tt, "account-page-flash-"+stage+"-"+fmt.Sprint(width))
					t.Fatalf("%s replayed or changed the underlying page", stage)
				}
			}
			checkMotion := func(stage string) {
				t.Helper()
				assertStable(stage)
				for _, elapsed := range []time.Duration{25, 45, 180} {
					tt.Advance(elapsed * time.Millisecond)
					assertStable(stage)
				}
			}
			clickUI(t, tt, a.tr().SideAccountManage)
			if !a.accountMenuOpen {
				t.Fatal("account menu did not open")
			}
			checkMotion("opening")
			tt.Key(0, ui.KeyEscape)
			tt.Advance(45 * time.Millisecond)
			assertStable("closing")
			clickUI(t, tt, a.tr().SideAccountManage)
			checkMotion("reopening")
			clickUI(t, tt, a.tr().SideAccountManage)
			checkMotion("closed")
		})
	}
}
