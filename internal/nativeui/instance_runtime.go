package nativeui

import (
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// loaderVersionOf extracts the loader build from standard installer ids:
// fabric-loader-X-MC, quilt-loader-X-MC, neoforge-X, MC-forge-X.
func loaderVersionOf(v launcher.VersionSummary) string {
	id, mc := v.ID, v.Minecraft
	switch loaderKey(v.Loader) {
	case "fabric", "quilt":
		for _, p := range []string{"fabric-loader-", "quilt-loader-"} {
			if strings.HasPrefix(id, p) && strings.HasSuffix(id, "-"+mc) {
				return strings.TrimSuffix(strings.TrimPrefix(id, p), "-"+mc)
			}
		}
	case "neoforge":
		if strings.HasPrefix(id, "neoforge-") {
			return strings.TrimPrefix(id, "neoforge-")
		}
	case "forge":
		if strings.HasPrefix(id, mc+"-forge-") {
			return strings.TrimPrefix(id, mc+"-forge-")
		}
	}
	return ""
}

// javaFor falls back to the parent vanilla version's requirement, since
// loader JSONs usually inherit javaVersion.
func javaFor(r launcher.Instance, v launcher.VersionSummary) int {
	if v.JavaVersion > 0 {
		return v.JavaVersion
	}
	for _, p := range r.Versions {
		if p.ID == v.Minecraft && p.JavaVersion > 0 {
			return p.JavaVersion
		}
	}
	return 0
}
