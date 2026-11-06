package launcher

import (
	"archive/zip"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// Immutable, content-addressed tables are shared by profiles of the same game.
// Each loader variant is built separately before publishing it to this cache.
var worldgenSymbolTables sync.Map

func (s *Service) seedMapSymbols(ctx context.Context, root string, v gameVersion) (*modjar.Symbols, error) {
	a, exists := v.Downloads["client_mappings"]
	if !exists {
		return nil, nil // Releases predating official mappings retain their native ABI.
	}
	if digest, err := hex.DecodeString(a.SHA1); err != nil || len(digest) != 20 || a.Size <= 0 || a.Size > 32<<20 {
		return nil, fmt.Errorf("invalid game mapping artifact")
	}
	var intermediary string
	var err error
	loader := installedLoader(v)
	for _, lib := range v.Libraries {
		if loader != "fabric" && loader != "quilt" {
			break
		}
		if !rulesAllow(lib.Rules) || !strings.HasPrefix(lib.Name, "net.fabricmc:intermediary:") {
			continue
		}
		artifact, e := libraryArtifact(lib)
		if e != nil {
			return nil, e
		}
		intermediary, err = safeJoin(root, "libraries/"+artifact.Path)
		if err != nil {
			return nil, err
		}
	}
	var srg string
	if version := versionArgument(v, "--fml.mcpVersion"); version != "" && (loader == "forge" || loader == "neoforge") {
		mc := versionArgument(v, "--fml.mcVersion")
		if mc == "" {
			mc = v.ClientVersion
		}
		name := mc + "-" + version
		srg, err = safeJoin(root, "libraries/de/oceanlabs/mcp/mcp_config/"+name+"/mcp_config-"+name+".zip")
		if err != nil {
			return nil, err
		}
	}
	key := a.SHA1
	for _, path := range []string{intermediary, srg} {
		if path == "" {
			continue
		}
		st, e := os.Stat(path)
		if e != nil {
			return nil, e
		}
		key += fmt.Sprintf("|%s:%d:%d", path, st.Size(), st.ModTime().UnixNano())
	}
	if cached, ok := worldgenSymbolTables.Load(key); ok {
		return cached.(*modjar.Symbols), ctx.Err()
	}
	path := filepath.Join(s.dir, "cache", "game-mappings", a.SHA1+".txt")
	if err := s.download(ctx, DownloadFile{Name: "game mappings", URL: a.URL, SHA1: a.SHA1, Size: a.Size}, path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	symbols, err := modjar.ReadOfficialSymbols(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	for _, input := range []struct{ path, resource string }{{intermediary, "mappings/mappings.tiny"}, {srg, "config/joined.tsrg"}} {
		if input.path == "" {
			continue
		}
		z, e := zip.OpenReader(input.path)
		if e != nil {
			return nil, e
		}
		stream, e := z.Open(input.resource)
		if e == nil {
			if input.path == intermediary {
				e = symbols.AddIntermediary(stream)
			} else {
				e = symbols.AddSRG(stream)
			}
			stream.Close()
		}
		z.Close()
		if e != nil {
			return nil, e
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actual, _ := worldgenSymbolTables.LoadOrStore(key, symbols)
	return actual.(*modjar.Symbols), nil
}
