package launcher

import "testing"

func TestDetectLoaderPrefersNeoForgeOverForgeHelpers(t *testing.T) {
	lib := func(names ...string) []library {
		var out []library
		for _, n := range names {
			out = append(out, library{Name: n})
		}
		return out
	}
	cases := []struct {
		name string
		v    gameVersion
		want string
	}{
		{"neoforge with srgutils", gameVersion{ID: "neoforge-21.1.264", Libraries: lib("net.neoforged:neoforge:21.1.264", "net.minecraftforge:srgutils:0.4.15")}, "neoforge"},
		{"srgutils listed first", gameVersion{ID: "x", Libraries: lib("net.minecraftforge:srgutils:0.4.15", "net.neoforged.fancymodloader:loader:4.0.1")}, "neoforge"},
		{"forge", gameVersion{ID: "1.20.1-forge-47.4.28", Libraries: lib("net.minecraftforge:fmlloader:1.20.1-47.4.28")}, "forge"},
		{"fabric", gameVersion{ID: "fabric-loader-0.20.1-1.21.8", Libraries: lib("net.fabricmc:fabric-loader:0.20.1")}, "fabric"},
		{"quilt", gameVersion{ID: "quilt-loader-0.20.0-1.21.8", Libraries: lib("org.quiltmc:quilt-loader:0.20.0", "net.fabricmc:intermediary:1.21.8")}, "quilt"},
		{"vanilla", gameVersion{ID: "1.21.8"}, "vanilla"},
		{"id fallback", gameVersion{ID: "neoforge-21.1.1"}, "neoforge"},
	}
	for _, c := range cases {
		if got := detectLoader(c.v); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}
