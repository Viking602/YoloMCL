package nativeui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/i18n"
	"github.com/Viking602/YoloMCL/internal/launcher"
)

// The precise count stays in the tooltip; the list uses the locale's familiar
// units. Truncating to one decimal avoids misleading boundaries like 1000K.
func downloadCountText(n int64, en bool) string {
	if text := i18n.CompactNumber(en, n); text != "" {
		return text
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
	count, hint, color := "—", a.L("version_metrics.download_count_unavailable_for_this_version"), muted
	if v.Downloads != nil && *v.Downloads >= 0 {
		count = downloadCountText(*v.Downloads, a.isEN())
		hint = fmt.Sprintf(a.L("version_metrics.downloads_of_this_version"), exactDownloadCount(*v.Downloads))
		color = inkSecondary
		if selected {
			color = accent
		}
	}
	var details, detailHints []string
	if relative, exact := a.resourceTime(c, v.Published); relative != "" {
		details = append(details, relative)
		detailHints = append(detailHints, fmt.Sprintf(a.L("version_metrics.published"), exact))
	}
	for _, file := range v.Files {
		if (file.Primary || len(v.Files) == 1) && file.Size > 0 {
			details = append(details, sizeText(file.Size))
			detailHints = append(detailHints, sizeText(file.Size))
			break
		}
	}
	ui.Column(c.Key("version-metrics")).Label(a.L("version_metrics.version_downloads")).Width(172 * s).Shrink(0).Gap(4 * s).AlignItems(ui.End).Children(func() {
		tooltip(c, ui.Row(c.Key("version-downloads")).Gap(5*s).AlignItems(ui.Center), hint).Children(func() {
			ui.Icon(c, icons["download"]).Size(11*s, 11*s).TextColor(color).Shrink(0)
			numText(c, count, typeCallout*s, color).FontWeight(650)
			ui.Text(c, a.L("version_metrics.downloads")).FontSize(typeCaption2 * s).TextColor(muted).NoWrap()
		})
		text := strings.Join(details, "  ")
		tooltip(c, numText(c, text, typeCaption*s, muted).SingleLine().FillWidth().TextAlign(ui.End), strings.Join(detailHints, "\n"))
	})
}
