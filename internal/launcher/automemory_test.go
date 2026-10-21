package launcher

import "testing"

func TestAutoMemoryMB(t *testing.T) {
	cases := []struct {
		name string
		in   AutoMemoryInput
		want int
	}{
		{"vanilla new 16G", AutoMemoryInput{TotalMB: 16384, AvailableMB: 10000, Minecraft: "1.21.8", Loader: "vanilla"}, 3072},
		{"vanilla 1.12 16G", AutoMemoryInput{TotalMB: 16384, AvailableMB: 10000, Minecraft: "1.12.2"}, 2048},
		{"vanilla 1.16 16G", AutoMemoryInput{TotalMB: 16384, AvailableMB: 10000, Minecraft: "1.16.5"}, 2560},
		{"fabric 0 mods", AutoMemoryInput{TotalMB: 16384, AvailableMB: 12000, Minecraft: "1.21.1", Loader: "fabric"}, 4096},
		{"forge 100 mods 32G", AutoMemoryInput{TotalMB: 32768, AvailableMB: 24000, Minecraft: "1.20.1", Loader: "forge", Mods: 100}, 7936},
		{"neoforge 300 mods capped 16G", AutoMemoryInput{TotalMB: 65536, AvailableMB: 60000, Minecraft: "1.21.1", Loader: "neoforge", Mods: 300}, 12288},
		{"huge pack hits max", AutoMemoryInput{TotalMB: 131072, AvailableMB: 120000, Minecraft: "1.21.1", Loader: "forge", Mods: 1000}, 12288},
		{"8G machine keeps OS room", AutoMemoryInput{TotalMB: 8192, AvailableMB: 7000, Minecraft: "1.20.1", Loader: "forge", Mods: 150}, 5632},
		{"low free memory", AutoMemoryInput{TotalMB: 16384, AvailableMB: 3000, Minecraft: "1.20.1", Loader: "fabric", Mods: 50}, 4096},
		{"4G machine floor", AutoMemoryInput{TotalMB: 4096, AvailableMB: 3500, Minecraft: "1.21"}, 1536},
		{"tiny machine min", AutoMemoryInput{TotalMB: 2048, AvailableMB: 500}, 1024},
		{"32-bit java", AutoMemoryInput{TotalMB: 16384, AvailableMB: 12000, Minecraft: "1.12.2", Loader: "forge", Mods: 50, Java32: true}, 1024},
		{"unknown RAM", AutoMemoryInput{Minecraft: "1.21.1"}, 3072},
		{"snapshot as new", AutoMemoryInput{TotalMB: 16384, AvailableMB: 12000, Minecraft: "25w14a"}, 3072},
	}
	for _, c := range cases {
		if got := AutoMemoryMB(c.in); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
		if got := AutoMemoryMB(c.in); got%256 != 0 || got < 1024 || got > 16384 {
			t.Errorf("%s: %d out of range", c.name, got)
		}
	}
}

func TestMemoryPlanChain(t *testing.T) {
	r := Root{MemoryMB: 0, VersionConfigs: map[string]VersionConfig{"m": {MemoryMB: 6144}, "a": {MemoryAuto: true}}}
	st := Settings{MemoryMB: 4096}
	if auto, _ := r.MemoryPlan("new", st); !auto {
		t.Error("new versions default to auto")
	}
	if auto, mb := r.MemoryPlan("m", st); auto || mb != 6144 {
		t.Error("manual version value must stay")
	}
	st.MemoryMode = "manual"
	if auto, mb := r.MemoryPlan("new", st); auto || mb != 4096 {
		t.Error("global manual")
	}
	if auto, _ := r.MemoryPlan("a", st); !auto {
		t.Error("explicit version auto wins")
	}
	st.UseRootSettings = true
	r.MemoryAuto = true
	if auto, _ := r.MemoryPlan("new", st); !auto {
		t.Error("directory auto")
	}
	r.MemoryMB = 3072
	if auto, mb := r.MemoryPlan("new", st); auto || mb != 3072 {
		t.Error("directory manual")
	}
	if !isJava32("x86") || !isJava32("i386") || isJava32("amd64") || isJava32("x86_64") || isJava32("") {
		t.Error("isJava32")
	}
}
