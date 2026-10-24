package nativeui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// The precise count stays in the tooltip; the list uses the locale's familiar
// units. Truncating to one decimal avoids misleading boundaries like 1000K.
func downloadCountText(n int64, en bool) string {
	units := []struct {
		size int64
		name string
	}{{100_000_000, "亿"}, {10_000, "万"}}
	if en {
		units = []struct {
			size int64
			name string
		}{{1_000_000_000, "B"}, {1_000_000, "M"}, {1_000, "K"}}
	}
	for _, unit := range units {
		if n >= unit.size {
			tenths := n / (unit.size / 10)
			if tenths%10 == 0 {
				return fmt.Sprintf("%d%s", tenths/10, unit.name)
			}
			return fmt.Sprintf("%d.%d%s", tenths/10, tenths%10, unit.name)
		}
	}
	return exactDownloadCount(n)
}

func exactDownloadCount(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func (a *App) versionMetrics(c *ui.Context, v launcher.ProjectVersion, selected bool) {
	s := uiScale(c)
	count, hint, color := "—", a.L("平台暂未提供此版本的下载次数", "Download count unavailable for this version"), muted
	if v.Downloads != nil && *v.Downloads >= 0 {
		count = downloadCountText(*v.Downloads, a.isEN())
		hint = fmt.Sprintf(a.L("此版本下载 %s 次", "%s downloads of this version"), exactDownloadCount(*v.Downloads))
		color = inkSecondary
		if selected {
			color = accent
		}
	}
	var details, detailHints []string
	if relative, exact := a.resourceTime(c, v.Published); relative != "" {
		details = append(details, relative)
		detailHints = append(detailHints, fmt.Sprintf(a.L("发布时间：%s", "Published: %s"), exact))
	}
	for _, file := range v.Files {
		if (file.Primary || len(v.Files) == 1) && file.Size > 0 {
			details = append(details, sizeText(file.Size))
			detailHints = append(detailHints, sizeText(file.Size))
			break
		}
	}
	ui.Column(c.Key("version-metrics")).Label(a.L("版本下载量", "Version downloads")).Width(172 * s).Shrink(0).Gap(4 * s).AlignItems(ui.End).Children(func() {
		ui.Row(c.Key("version-downloads")).Gap(5 * s).AlignItems(ui.Center).Tooltip(hint).Children(func() {
			ui.Icon(c, icons["download"]).Size(11*s, 11*s).TextColor(color).Shrink(0)
			numText(c, count, typeCallout*s, color).FontWeight(650)
			ui.Text(c, a.L("下载", "downloads")).FontSize(typeCaption2 * s).TextColor(muted).NoWrap()
		})
		text := strings.Join(details, "  ")
		numText(c, text, typeCaption*s, muted).SingleLine().FillWidth().TextAlign(ui.End).Tooltip(strings.Join(detailHints, "\n"))
	})
}
