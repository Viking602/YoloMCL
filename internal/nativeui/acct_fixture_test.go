package nativeui

import (
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

var acctFixture = []launcher.Account{
	{ID: "ms-1", Name: "IceTheGreat", UUID: "069a79f444e94726a5befca90e38aaf5", Kind: "microsoft"},
	{ID: "off-1", Name: "Steve", UUID: "00000000000000000000000000000000", Kind: "offline"},
	{ID: "off-2", Name: "AVeryVeryLongOfflinePlayerNameThatGoesOnAndOn_1234567890", UUID: "00000000000000000000000000000001", Kind: "offline"},
	{ID: "yg-1", Name: "LittleSkinUser", UUID: "11111111111111111111111111111111", Kind: "yggdrasil", Server: "https://littleskin.cn/api/yggdrasil"},
	{ID: "yg-2", Name: "第三方超长名字的玩家角色名称测试", UUID: "22222222222222222222222222222222", Kind: "yggdrasil", Server: "https://skin.some-very-long-community-server-domain.example.org/api/yggdrasil"},
}

// acctTester opens the account dialog with a mixed fixture of accounts.
func acctTester(t *testing.T, lang string, w, h int) (*App, *ui.Tester) {
	t.Helper()
	a, tt := homeTester(t, lang)
	tt.SetSize(w, h)
	apply := func() {
		a.snapshot.Instances, a.snapshot.ActiveRoot = homeFixture, "r1"
		a.snapshot.Accounts, a.snapshot.ActiveAccount = acctFixture, "off-1"
		a.snapshot.AuthServers = append(append([]launcher.AuthServer{}, launcher.BuiltinAuthServers...), launcher.AuthServer{Name: "某个名字很长的社区皮肤站 Community Skin Server", URL: "https://skin.some-very-long-community-server-domain.example.org/api/yggdrasil"})
	}
	// no network in UI tests: favicons are "tried" (→ globe) unless injected
	a.avatars.icons, a.avatars.iconTried = map[string]*ui.Bitmap{}, map[string]bool{}
	for _, ac := range acctFixture {
		if ac.Server != "" {
			a.avatars.iconTried[strings.ToLower(ac.Server)] = true
		}
	}
	apply()
	a.openDialog("account")
	for i := 0; i < 4; i++ {
		tt.Frame()
		apply()
	}
	tt.Frame()
	return a, tt
}

func TestAccountListNoOverflow(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		for _, size := range [][2]int{{1536, 1024}, {1100, 760}, {900, 600}} {
			a, tt := acctTester(t, lang, size[0], size[1])
			modal, ok := tt.Find("活动弹窗")
			if !ok {
				t.Fatal("dialog missing")
			}
			found := 0
			for _, ac := range acctFixture {
				row, ok := tt.Find("账号行 " + ac.ID)
				if !ok {
					continue // scrolled
				}
				found++
				if row.X < modal.X || row.X+row.W > modal.X+modal.W+0.5 {
					t.Errorf("%s %v: row %s escapes dialog", lang, size, ac.ID)
				}
				// names/tags that are unique on screen (the active name also
				// appears in the top-right pill; 离线/正版 tags repeat)
				uniq := []string{}
				if ac.ID != a.snapshot.ActiveAccount {
					uniq = append(uniq, ac.Name)
				}
				if ac.Kind == "yggdrasil" {
					uniq = append(uniq, a.accountKindTag(ac))
				}
				for _, txt := range uniq {
					r, ok := tt.Find(txt)
					if !ok || r.X < row.X-0.5 || r.X+r.W > row.X+row.W+0.5 || r.Y < row.Y-0.5 || r.Y+r.H > row.Y+row.H+0.5 {
						t.Errorf("%s %v: %q overflows row %s: %+v in %+v", lang, size, txt, ac.ID, r, row)
					}
				}
			}
			if found < 2 {
				t.Errorf("%s %v: account rows not rendered (%d)", lang, size, found)
			}
			if !tt.HasText(a.tr().AccTagThird + " · LittleSkin") {
				t.Errorf("%s: third-party tag should include the saved server name", lang)
			}
			if tt.HasText(a.tr().AccMSDesc) || tt.HasText(a.tr().AccOfflineDesc) {
				t.Errorf("%s: add form should be collapsed while accounts exist", lang)
			}
		}
	}
}

func TestThirdPartyServerPickerAndValidation(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1536, 1024)
	tr := a.tr()
	clickUI(t, tt, tr.AccAddAccountBtn)
	clickUI(t, tt, tr.AccTabThirdParty)
	tt.Frame()
	if a.form.Server != "https://littleskin.cn/api/yggdrasil" {
		t.Errorf("first saved server should pre-fill the address: %q", a.form.Server)
	}
	if !tt.HasText("LittleSkin") || !tt.HasText(tr.AccAddServerBtn) {
		t.Error("server picker missing")
	}
	clickUI(t, tt, tr.AccAddServerBtn)
	tt.Frame()
	a.serverEdit.url = "http://insecure.example.com"
	tt.Frame()
	clickUI(t, tt, tr.AccServerSaveBtn)
	tt.Frame()
	if a.serverEdit == nil || a.serverEdit.err != tr.AccServerURLInvalid {
		t.Error("http address must be rejected")
	}
}

func TestAccountTypeTagsDistinct(t *testing.T) {
	cols := map[ui.Color]string{}
	for _, k := range []string{"microsoft", "offline", "yggdrasil"} {
		c := accountTypeColor(k)
		if prev, dup := cols[c]; dup {
			t.Errorf("%s shares colour with %s", k, prev)
		}
		cols[c] = k
	}
	for _, lang := range []string{"zh-CN", "en-US"} {
		a, tt := acctTester(t, lang, 1536, 1024)
		for _, k := range []string{"microsoft", "offline", "yggdrasil"} {
			tag, ok := tt.Find("类型标签 " + k)
			icon, ok2 := tt.Find("类型图标 " + k)
			if !ok || !ok2 || icon.X < tag.X || icon.X+icon.W > tag.X+tag.W {
				t.Errorf("%s: %s tag/icon missing or misplaced: %+v %+v", lang, k, tag, icon)
			}
		}
		clickUI(t, tt, a.tr().AccAddAccountBtn)
		tt.Frame()
		for _, k := range []string{"microsoft", "offline", "yggdrasil"} {
			if _, ok := tt.Find("类型图标 " + k); !ok {
				t.Errorf("%s: segmented control lacks %s icon", lang, k)
			}
		}
	}
}

func TestAuthFaviconReplacesGlobe(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1536, 1024)
	if _, ok := tt.Find("类型图标 yggdrasil"); !ok {
		t.Fatal("custom server without icon should show globe")
	}
	a.avatars.icons["https://skin.some-very-long-community-server-domain.example.org/api/yggdrasil"] = ui.NewBitmap(image.NewNRGBA(image.Rect(0, 0, 32, 32)))
	tt.Frame()
	if _, ok := tt.Find("类型图标 yggdrasil"); ok {
		t.Error("cached favicon should replace the globe")
	}
}

func TestBuiltinServersInPicker(t *testing.T) {
	a, tt := acctTester(t, "zh-CN", 1536, 1024)
	tr := a.tr()
	clickUI(t, tt, tr.AccAddAccountBtn)
	clickUI(t, tt, tr.AccTabThirdParty)
	tt.Frame()
	if a.form.Server != "https://littleskin.cn/api/yggdrasil" {
		t.Errorf("default should be LittleSkin: %q", a.form.Server)
	}
	if _, ok := tt.Find("内置标记 LittleSkin"); !ok {
		t.Error("built-in marker missing")
	}
	if tt.HasText(tr.AccEditServerBtn) || tt.HasText(tr.AccDeleteServerBtn) {
		t.Error("built-in must hide edit/delete")
	}
	if bm := a.authIcon("https://littleskin.cn/api/yggdrasil/"); bm == nil || a.authIcon("https://ely.by") == nil {
		t.Error("bundled icons missing")
	}
	// a typed site root maps onto the built-in
	a.form.Server = "http://ely.by/"
	tt.Frame()
	if a.form.Server != "https://account.ely.by/api/authlib-injector" {
		t.Errorf("ely root not resolved: %q", a.form.Server)
	}
	if a.accountServerName("https://littleskin.cn") != "LittleSkin" {
		t.Error("account linked name")
	}
}
