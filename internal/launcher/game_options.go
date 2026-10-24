package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GameOptions is what the game itself remembers for an instance: the
// shader pack picked in Iris / Oculus / OptiFine and the resource pack
// order from options.txt. Only files that exist are read; unknown stays "".
type GameOptions struct {
	ShaderConfig      string // iris | oculus | optifine | ""
	ShaderPack        string
	ShadersOn         bool
	HasOptions        bool
	ResourcePackOrder []string // pack files, highest priority first
}

func parseProps(b []byte) map[string]string {
	out := map[string]string{}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		for strings.HasSuffix(line, `\`) && !strings.HasSuffix(line, `\\`) && i+1 < len(lines) { // continuation
			i++
			line = strings.TrimSuffix(line, `\`) + strings.TrimSpace(lines[i])
		}
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			k, v, ok = strings.Cut(line, ":")
		}
		if !ok {
			continue
		}
		out[propUnescape(strings.TrimSpace(k))] = propUnescape(strings.TrimSpace(v))
	}
	return out
}

// propUnescape decodes Java .properties escapes: \uXXXX (Iris/Oculus/
// OptiFine write non-ASCII names this way), \t \n \r \f, and \x -> x for
// any other char (\! \: \= \# \space \\).
func propUnescape(v string) string {
	if !strings.Contains(v, `\`) {
		return v
	}
	var b strings.Builder
	r := []rune(v)
	for i := 0; i < len(r); i++ {
		if r[i] != '\\' || i+1 >= len(r) {
			b.WriteRune(r[i])
			continue
		}
		i++
		switch r[i] {
		case 'u':
			if i+4 < len(r) {
				if n, e := strconv.ParseUint(string(r[i+1:i+5]), 16, 32); e == nil {
					b.WriteRune(rune(n))
					i += 4
					continue
				}
			}
			b.WriteRune('u')
		case 't':
			b.WriteRune('\t')
		case 'n':
			b.WriteRune('\n')
		case 'r':
			b.WriteRune('\r')
		case 'f':
			b.WriteRune('\f')
		default:
			b.WriteRune(r[i])
		}
	}
	return b.String()
}

// ReadGameOptions reads only the selected instance's optional game settings.
// Missing files mean unknown/default state; unreadable or oversized files fail.
func (s *Service) ReadGameOptions(ctx context.Context, rootID, versionID string) (GameOptions, error) {
	dir, err := s.InstanceFolder(rootID, versionID, "")
	if err != nil {
		return GameOptions{}, err
	}
	return readGameOptions(ctx, dir)
}

const maxGameOptionsSize = 1 << 20

func readOptionalGameFile(ctx context.Context, dir, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxGameOptionsSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxGameOptionsSize {
		return nil, fmt.Errorf("game options file %s exceeds 1 MiB", name)
	}
	return data, ctx.Err()
}

func readGameOptions(ctx context.Context, dir string) (GameOptions, error) {
	var st GameOptions
	for _, c := range []struct{ file, kind string }{{"config/iris.properties", "iris"}, {"config/oculus.properties", "oculus"}} {
		b, e := readOptionalGameFile(ctx, dir, c.file)
		if e != nil {
			return st, e
		}
		if b == nil {
			continue
		}
		p := parseProps(b)
		st.ShaderConfig, st.ShaderPack = c.kind, p["shaderPack"]
		st.ShadersOn = st.ShaderPack != "" && !strings.EqualFold(p["enableShaders"], "false")
		break
	}
	if st.ShaderConfig == "" {
		b, e := readOptionalGameFile(ctx, dir, "optionsshaders.txt")
		if e != nil {
			return st, e
		}
		if b != nil {
			st.ShaderConfig = "optifine"
			if v := parseProps(b)["shaderPack"]; v != "" && v != "OFF" && v != "(internal)" {
				st.ShaderPack, st.ShadersOn = v, true
			}
		}
	}
	b, e := readOptionalGameFile(ctx, dir, "options.txt")
	if e != nil {
		return st, e
	}
	if b != nil {
		st.HasOptions = true
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "resourcePacks:"); ok {
				var list []string
				if err := json.Unmarshal([]byte(v), &list); err != nil {
					return st, fmt.Errorf("parse resourcePacks in options.txt: %w", err)
				}
				for i := len(list) - 1; i >= 0; i-- { // the last entry wins in game
					if f, ok := strings.CutPrefix(list[i], "file/"); ok {
						st.ResourcePackOrder = append(st.ResourcePackOrder, f)
					}
				}
			}
		}
	}
	return st, nil
}
