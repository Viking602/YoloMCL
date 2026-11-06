package structure3d

import (
	"errors"
	"fmt"
	"image"
)

// Error kinds let the UI show a localized message instead of raw Go errors.
const (
	KindMissingStructure = "missing_structure" // no worldgen/structure json in any layer
	KindMissingTemplate  = "missing_template"  // a referenced .nbt is in no layer
	KindBadTemplate      = "bad_template"      // .nbt failed to decode
	KindBadStructure     = "bad_structure"     // structure json failed to parse
	KindCodeGenerated    = "code_generated"    // non-jigsaw type placed by Java code
	KindUnsupportedPiece = "unsupported_piece" // pool element type we do not draw
	KindEmpty            = "empty"             // assembly produced no blocks
)

// PreviewError is a failed preview with a kind the UI can localize.
type PreviewError struct {
	Kind string
	ID   string
	Err  error
}

func (e *PreviewError) Error() string {
	return fmt.Sprintf("structure3d: %s %s: %v", e.Kind, e.ID, e.Err)
}

func (e *PreviewError) Unwrap() error { return e.Err }

// PreviewRequest describes one structure preview.
type PreviewRequest struct {
	StructureID   string // registry id, e.g. minecraft:village_plains
	TemplateID    string // optional direct template (single-piece structures)
	Seed          int64
	X, Z          int
	Width, Height int
	Camera        Camera
	Surface       HeightFunc
	Dimension     string // seedmap dimension id; "" is the overworld
	// Ground, when set, is the pre-structure terrain the code-built pieces
	// (fortress, mineshaft, stronghold, temples) test for air, liquid and
	// support; nil estimates it from Surface.
	Ground Ground
	// SeaLevel, when non-zero, says Surface is the solid noise terrain and
	// the dimension's sea surface lies at this Y (see AssembleGroundSea).
	SeaLevel int
}

// DimHeightFor is the build height of a vanilla dimension; other ids use
// the overworld's.
func DimHeightFor(dimension string) DimHeight {
	switch dimension {
	case "minecraft:the_nether", "the_nether", "nether", "minecraft:the_end", "the_end", "end":
		return DimHeight{MinY: 0, Height: 256}
	}
	return OverworldHeight
}

// PreviewResult is a rendered frame plus diagnostics.
type PreviewResult struct {
	Image           *image.RGBA
	BlockCount      int
	Unsupported     string // non-empty => UI shows 暂时不支持; raw reason for tooltip/log
	UnsupportedKind string // KindCodeGenerated, KindUnsupportedPiece, KindEmpty
	Title           string
	LayoutSeeded    bool  // piece layout seed-exact, blocks illustrative
	Seeded          bool  // seed-exact placement (no approximation note)
	Mesh            *Mesh // baked scene: re-render a new camera without reassembling
}

// Preview assembles and renders. It never invents geometry for unsupported pieces.
func Preview(store *Store, req PreviewRequest) (*PreviewResult, error) {
	if req.Width <= 0 {
		req.Width = 640
	}
	if req.Height <= 0 {
		req.Height = 480
	}
	if req.Camera.Zoom == 0 {
		req.Camera = DefaultCamera()
	}
	var scene *Scene
	var err error
	title := req.StructureID
	switch {
	case req.TemplateID != "":
		title = req.TemplateID
		scene, err = AssembleTemplate(store, req.TemplateID, req.X, 0, req.Z)
	case req.StructureID != "":
		scene, err = AssembleGroundSea(store, req.StructureID, req.Seed, req.X, req.Z, req.Surface, DimHeightFor(req.Dimension), req.Ground, req.SeaLevel)
	default:
		return nil, fmt.Errorf("structure3d: missing structure id")
	}
	if err != nil {
		var u *UnsupportedError
		if errors.As(err, &u) {
			return &PreviewResult{Unsupported: u.Reason, UnsupportedKind: u.Kind, Title: title}, nil
		}
		return nil, err
	}
	mesh := BuildMesh(store, scene)
	img := mesh.Render(&Frame{}, req.Width, req.Height, req.Camera)
	if !hasDrawnPixel(img) {
		// No resolved faces were visible: report it rather than show a blank stage.
		return &PreviewResult{Unsupported: fmt.Sprintf("%d blocks, none with an opaque model", len(scene.Blocks)), UnsupportedKind: KindEmpty, Title: title, BlockCount: len(scene.Blocks)}, nil
	}
	return &PreviewResult{Image: img, BlockCount: len(scene.Blocks), Title: title, Mesh: mesh, Seeded: scene.Seeded, LayoutSeeded: scene.LayoutSeeded}, nil
}

func hasDrawnPixel(img *image.RGBA) bool {
	for i := 0; i+2 < len(img.Pix); i += 4 {
		if img.Pix[i] != 0xe8 || img.Pix[i+1] != 0xec || img.Pix[i+2] != 0xef {
			return true
		}
	}
	return false
}
