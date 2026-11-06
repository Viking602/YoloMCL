package i18n

import "testing"

func TestBodyTextBindings(t *testing.T) {
	for _, tc := range [][2]string{
		{"512 MB / 1.5 MiB/s / 20 ms", "512\u00a0MB / 1.5\u00a0MiB/s / 20\u00a0ms"},
		{"容量 8 GB，包含 32 个方块。", "容量 8\u00a0GB，包含 32\u00a0个方块。"},
		{"Creator — Alex", "Creator —\u00a0Alex"},
		{"Minecraft 1.21.1 needs Java 21 or later", "Minecraft 1.21.1 needs Java 21 or later"},
		{"12 models / 32 Minecraft versions", "12 models / 32 Minecraft versions"},
		{"512\u00a0MB\n😀 —\u00a0Alex", "512\u00a0MB\n😀 —\u00a0Alex"},
	} {
		if got := BindTextPairs(tc[0]); got != tc[1] {
			t.Errorf("BindTextPairs(%q) = %q, want %q", tc[0], got, tc[1])
		}
	}
}
