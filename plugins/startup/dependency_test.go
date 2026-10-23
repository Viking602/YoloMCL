package startup

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPinnedDependencyAndShaders(t *testing.T) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}|{{.Dir}}|{{if .Replace}}replaced{{end}}", "github.com/egoist/mygo")
	b, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(strings.TrimSpace(string(b)), "|")
	if len(parts) != 3 || parts[0] != mygoVersion || parts[2] != "" {
		t.Fatalf("review startup compatibility for changed MyGo dependency: %s", strings.TrimSpace(string(b)))
	}
	root := parts[1]
	f, e := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal", "platform", "surface.go"), nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	got := map[string]int{}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for i, s := range g.Specs {
			v := s.(*ast.ValueSpec)
			for _, n := range v.Names {
				if n.Name == "SurfaceFrame" || n.Name == "SurfaceRenew" {
					got[n.Name] = i
				}
			}
		}
	}
	if got["SurfaceFrame"] != surfaceFrame || got["SurfaceRenew"] != surfaceRenew {
		t.Fatalf("surface event contract changed: %v", got)
	}
	f, e = parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal", "gpu", "d3d11", "shaders.go"), nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]string{"vertexShader": "vertex.dxbc", "pixelShader": "pixel.dxbc"}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, s := range g.Specs {
			v := s.(*ast.ValueSpec)
			if len(v.Names) != 1 || len(v.Values) != 1 {
				continue
			}
			file, ok := want[v.Names[0].Name]
			if !ok {
				continue
			}
			arr := v.Values[0].(*ast.CompositeLit)
			code := make([]byte, len(arr.Elts))
			for i, el := range arr.Elts {
				n, e := strconv.ParseUint(el.(*ast.BasicLit).Value, 0, 8)
				if e != nil {
					t.Fatal(e)
				}
				code[i] = byte(n)
			}
			stored, e := os.ReadFile(filepath.Join("assets", "mygo-0.5.5", file))
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(stored, code) {
				t.Fatalf("shader asset %s differs from pinned dependency", file)
			}
			delete(want, v.Names[0].Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing upstream shaders: %v", want)
	}
}
