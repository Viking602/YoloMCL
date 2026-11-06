package modjar

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Symbols joins a game's official mappings with its loader's namespace. The
// table is data supplied by the selected game, never a list of mod aliases.
// Keys include the owner and full descriptor, so overloaded methods and fields
// with the same short name remain distinct. A table is immutable after loading.
type Symbols struct {
	Classes map[string]string
	Members map[string]string
	// unique holds globally numbered names (SRG m_N_/f_N_, intermediary
	// method_N/field_N) by name and descriptor, so an override declared in a
	// mod class maps like the game member it overrides.
	unique map[string]string
}

func (s *Symbols) class(name string) string {
	if s != nil {
		if mapped := s.Classes[name]; mapped != "" {
			return mapped
		}
	}
	return name
}

func (s *Symbols) descriptor(desc string) string {
	if s == nil {
		return desc
	}
	var b strings.Builder
	for len(desc) > 0 {
		i := strings.IndexByte(desc, 'L')
		if i < 0 {
			b.WriteString(desc)
			break
		}
		b.WriteString(desc[:i+1])
		desc = desc[i+1:]
		end := strings.IndexByte(desc, ';')
		if end < 0 {
			b.WriteString(desc)
			break
		}
		b.WriteString(s.class(desc[:end]))
		b.WriteByte(';')
		desc = desc[end+1:]
	}
	return b.String()
}

func (s *Symbols) typeName(name string) string {
	if strings.HasPrefix(name, "[") {
		return s.descriptor(name)
	}
	return s.class(name)
}

func (s *Symbols) ref(r reference) reference {
	if s == nil {
		return r
	}
	r.owner, r.desc = s.class(r.owner), s.descriptor(r.desc)
	if name := s.Members[fieldKey(r)]; name != "" {
		r.name = name
	} else if numberedName(r.name) {
		if name := s.unique[r.name+r.desc]; name != "" {
			r.name = name
		}
	}
	return r
}

func (s *Symbols) member(owner, name, desc, target string) error {
	r := reference{s.class(owner), name, s.descriptor(desc)}
	key := fieldKey(r)
	if old := s.Members[key]; old != "" && old != target {
		return fmt.Errorf("conflicting game member mapping %s", key)
	}
	s.Members[key] = target
	if numberedName(name) {
		if s.unique == nil {
			s.unique = make(map[string]string)
		}
		s.unique[name+r.desc] = target
	}
	return nil
}

func mappingLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(io.LimitReader(r, (32<<20)+1))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var lines []string
	size := 0
	for scanner.Scan() {
		line := scanner.Text()
		size += len(line) + 1
		if size > 32<<20 || len(lines) >= 1000000 {
			return nil, fmt.Errorf("game mapping size limit")
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

func javaDescriptor(name string) (string, error) {
	prefix := ""
	for strings.HasSuffix(name, "[]") {
		prefix += "["
		name = strings.TrimSuffix(name, "[]")
	}
	primitives := map[string]string{"void": "V", "boolean": "Z", "byte": "B", "char": "C", "short": "S", "int": "I", "long": "J", "float": "F", "double": "D"}
	if d := primitives[name]; d != "" {
		return prefix + d, nil
	}
	if name == "" || strings.ContainsAny(name, " ();[]") {
		return "", fmt.Errorf("invalid mapped Java type %q", name)
	}
	return prefix + "L" + strings.ReplaceAll(name, ".", "/") + ";", nil
}

// ReadOfficialSymbols reads the named -> obfuscated ProGuard map published
// with the selected Minecraft version, reversing it for the native API host.
func ReadOfficialSymbols(r io.Reader) (*Symbols, error) {
	lines, err := mappingLines(r)
	if err != nil {
		return nil, err
	}
	s := &Symbols{Classes: map[string]string{}, Members: map[string]string{}}
	for _, line := range lines {
		if line == "" || line[0] == '#' || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		left, right, ok := strings.Cut(strings.TrimSuffix(line, ":"), " -> ")
		if !ok || !strings.HasSuffix(line, ":") {
			return nil, fmt.Errorf("invalid game class mapping")
		}
		s.Classes[right] = strings.ReplaceAll(left, ".", "/")
	}
	var owner string
	for _, line := range lines {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			_, owner, _ = strings.Cut(strings.TrimSuffix(line, ":"), " -> ")
			continue
		}
		left, alias, ok := strings.Cut(strings.TrimSpace(line), " -> ")
		if !ok || owner == "" {
			return nil, fmt.Errorf("invalid game member mapping")
		}
		// Source line ranges are optional and are not part of a descriptor.
		for len(left) > 0 && left[0] >= '0' && left[0] <= '9' {
			_, rest, ok := strings.Cut(left, ":")
			if !ok {
				return nil, fmt.Errorf("invalid mapped line range")
			}
			left = rest
		}
		typ, signature, ok := strings.Cut(left, " ")
		if !ok {
			return nil, fmt.Errorf("invalid mapped signature")
		}
		desc, err := javaDescriptor(typ)
		if err != nil {
			return nil, err
		}
		name := signature
		if i := strings.IndexByte(signature, '('); i >= 0 {
			name = signature[:i]
			end := strings.IndexByte(signature[i:], ')')
			if end < 0 {
				return nil, fmt.Errorf("invalid mapped method")
			}
			args := signature[i+1 : i+end]
			parameters := ""
			if args != "" {
				for _, arg := range strings.Split(args, ",") {
					d, err := javaDescriptor(arg)
					if err != nil {
						return nil, err
					}
					parameters += d
				}
			}
			desc = "(" + parameters + ")" + desc
		}
		// Qualified names describe inlined frames, not members of this class.
		if strings.Contains(name, ".") {
			continue
		}
		if err := s.member(owner, alias, desc, name); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// AddIntermediary joins Fabric's distributed Tiny v1 official/intermediary
// mapping to the official table. Descriptor classes are translated as a unit.
func (s *Symbols) AddIntermediary(r io.Reader) error {
	lines, err := mappingLines(r)
	if err != nil {
		return err
	}
	if len(lines) == 0 || lines[0] != "v1\tofficial\tintermediary" {
		return fmt.Errorf("unsupported intermediary mapping header")
	}
	for _, line := range lines[1:] {
		p := strings.Split(line, "\t")
		if len(p) == 3 && p[0] == "CLASS" {
			s.Classes[p[2]] = s.class(p[1])
		}
	}
	for _, line := range lines[1:] {
		p := strings.Split(line, "\t")
		if len(p) != 5 || p[0] != "FIELD" && p[0] != "METHOD" {
			continue
		}
		target := s.ref(reference{p[1], p[3], p[2]})
		if err := s.member(target.owner, p[4], target.desc, target.name); err != nil {
			return err
		}
	}
	return nil
}

// AddSRG joins MCP's TSRG2 table. Fields have no descriptors in TSRG2, so all
// matching official field descriptors are retained rather than guessing one.
func (s *Symbols) AddSRG(r io.Reader) error {
	lines, err := mappingLines(r)
	if err != nil {
		return err
	}
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "tsrg2 ") {
		return fmt.Errorf("unsupported SRG mapping header")
	}
	fields := map[string]map[string]string{}
	for key, target := range s.Members {
		i := strings.LastIndexByte(key, ':')
		if i < 0 || strings.HasPrefix(key[i+1:], "(") {
			continue
		}
		if fields[key[:i]] == nil {
			fields[key[:i]] = map[string]string{}
		}
		fields[key[:i]][key[i+1:]] = target
	}
	var owner string
	for _, line := range lines[1:] {
		if line == "" || strings.HasPrefix(line, "\t\t") {
			continue
		}
		p := strings.Fields(line)
		if line[0] != '\t' {
			if len(p) < 2 {
				return fmt.Errorf("invalid SRG class mapping")
			}
			owner = p[0]
			s.Classes[p[1]] = s.class(owner)
			continue
		}
		if len(p) < 2 || owner == "" {
			return fmt.Errorf("invalid SRG member mapping")
		}
		if strings.HasPrefix(p[1], "(") {
			if len(p) < 3 {
				return fmt.Errorf("invalid SRG method mapping")
			}
			target := s.ref(reference{owner, p[0], p[1]})
			if err := s.member(target.owner, p[2], target.desc, target.name); err != nil {
				return err
			}
		} else {
			for desc, target := range fields[s.class(owner)+"."+p[0]] {
				if err := s.member(owner, p[1], desc, target); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func numberedName(n string) bool {
	var digits string
	switch {
	case len(n) > 3 && (n[:2] == "m_" || n[:2] == "f_") && n[len(n)-1] == '_':
		digits = n[2 : len(n)-1]
	case strings.HasPrefix(n, "method_"):
		digits = n[7:]
	case strings.HasPrefix(n, "field_"):
		digits = n[6:]
	default:
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return digits != ""
}
