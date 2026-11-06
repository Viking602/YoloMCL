package structure3d

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// lithoTemplatesClass marks Lithostitched builds whose JigsawStructureMixin
// reroutes minecraft:jigsaw placement to AlternateJigsawGenerator.
const lithoTemplatesClass = "dev/worldgen/lithostitched/worldgen/structure/LithostitchedTemplates.class"

// DetectLithostitched turns on the Lithostitched jigsaw generator when a
// loaded mod ships it and gameDir/config/lithostitched.json does not set
// breaks_seed_parity to false (the mod's default is true). Call after mods
// are opened. Reports whether it was enabled.
func DetectLithostitched(s *Store, gameDir string) bool {
	if _, err := s.Read(lithoTemplatesClass); err != nil {
		return false
	}
	on := true
	if b, err := os.ReadFile(filepath.Join(gameDir, "config", "lithostitched.json")); err == nil {
		var cfg struct {
			BreaksSeedParity *bool `json:"breaks_seed_parity"`
		}
		if json.Unmarshal(lenientJSON(b), &cfg) == nil && cfg.BreaksSeedParity != nil {
			on = *cfg.BreaksSeedParity
		}
	}
	s.SetLithostitchedJigsaw(on)
	return on
}
