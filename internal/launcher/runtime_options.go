package launcher

import (
	"errors"
	"fmt"
	"strings"
)

// ParseJVMArgs splits quoted JVM options and rejects classpath/main-JAR overrides.
func ParseJVMArgs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.ContainsRune(raw, 0) {
		return nil, errors.New("JVM 参数包含无效字符")
	}
	var out []string
	var cur strings.Builder
	var quote rune
	inToken := false
	for _, r := range raw {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if inToken {
				out = append(out, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, errors.New("JVM 参数引号未闭合")
	}
	if inToken {
		out = append(out, cur.String())
	}
	for _, tok := range out {
		low := strings.ToLower(tok)
		if low == "-cp" || low == "-classpath" || low == "--class-path" || low == "-jar" {
			return nil, fmt.Errorf("不支持在附加 JVM 参数中覆盖类路径或主 JAR: %s", tok)
		}
	}
	return out, nil
}

// ResolveRuntime applies version overrides, optional root defaults and global settings.
func (r Root) ResolveRuntime(versionID string, settings Settings) (string, int, []string, error) {
	vc := r.VersionConfigs[versionID]
	javaPath := strings.TrimSpace(vc.JavaPath)
	if javaPath == "" && settings.UseRootSettings {
		javaPath = strings.TrimSpace(r.JavaPath)
	}
	if javaPath == "" {
		javaPath = strings.TrimSpace(settings.JavaPath)
	}
	// memoryMB 0 means automatic (see MemoryPlan / AutoMemoryFor).
	_, memoryMB := r.MemoryPlan(versionID, settings)
	rawJVM := strings.TrimSpace(vc.JVMArgs)
	if rawJVM == "" && settings.UseRootSettings {
		rawJVM = strings.TrimSpace(r.JVMArgs)
	}
	if rawJVM == "" {
		rawJVM = strings.TrimSpace(settings.JVMArgs)
	}
	jvmArgs, err := ParseJVMArgs(rawJVM)
	return javaPath, memoryMB, jvmArgs, err
}

func validateOptionalMemory(memoryMB int) error {
	if memoryMB == 0 {
		return nil
	}
	if memoryMB < 512 || memoryMB > 131072 {
		return errors.New("内存设置无效")
	}
	if info, e := SystemMemory(); e == nil && memoryMB > info.MaxMB {
		return fmt.Errorf("设置内存超过系统内存上限 %d MB", info.MaxMB)
	}
	return nil
}
