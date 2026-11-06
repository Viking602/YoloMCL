package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
)

// BiomeSourceContract describes a class-loader ABI, not a reconstructed source
// program or a promise that the generator can be executed. Resource IDs must
// still be resolved by the game's actual codec registration.
type BiomeSourceContract struct {
	Class, Super                     string
	Constructors, CodecFields, Calls []string
}

// InspectBiomeSources finds concrete biome-source API declarations in a JAR.
// It retains method signatures and required calls, never Java source or code
// dumps. The same bounded parser is used by the Go registration interpreter.
func InspectBiomeSources(ctx context.Context, z *zip.Reader) (result []BiomeSourceContract, err error) {
	defer func() {
		if p := recover(); p != nil {
			result = nil
			err = fmt.Errorf("biome source contract: %v", p)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var total uint64
	for _, entry := range z.File {
		if !strings.HasSuffix(entry.Name, ".class") || strings.HasPrefix(entry.Name, "META-INF/") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[entry.Name] || len(seen) >= 65536 {
			return nil, fmt.Errorf("duplicate class or inspection class limit")
		}
		seen[entry.Name] = true
		total += entry.UncompressedSize64
		if entry.UncompressedSize64 > 2<<20 || total > 256<<20 {
			return nil, fmt.Errorf("biome source inspection byte limit")
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(stream, (2<<20)+1))
		stream.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > 2<<20 {
			return nil, fmt.Errorf("biome source class size limit")
		}
		if !bytes.Contains(data, []byte("net/minecraft/world/level/biome/")) {
			continue
		}
		c := parseClass(data)
		if c.name+".class" != entry.Name {
			return nil, fmt.Errorf("biome source class entry mismatch")
		}
		if !strings.HasPrefix(c.super, "net/minecraft/world/level/biome/") || !strings.HasSuffix(c.super, "BiomeSource") {
			continue
		}
		contract := BiomeSourceContract{Class: c.name, Super: c.super}
		for _, field := range c.fields {
			if field.access&8 != 0 && (field.desc == "Lcom/mojang/serialization/Codec;" || field.desc == "Lcom/mojang/serialization/MapCodec;") {
				contract.CodecFields = append(contract.CodecFields, field.name+":"+field.desc)
			}
		}
		calls := map[string]bool{}
		for _, m := range c.methods {
			if m.name == "<init>" {
				contract.Constructors = append(contract.Constructors, m.desc)
			}
			for _, r := range methodReferences(c, m) {
				calls[r.owner+"."+r.name+r.desc] = true
			}
		}
		for call := range calls {
			contract.Calls = append(contract.Calls, call)
		}
		slices.Sort(contract.Calls)
		slices.Sort(contract.Constructors)
		slices.Sort(contract.CodecFields)
		result = append(result, contract)
	}
	slices.SortFunc(result, func(a, b BiomeSourceContract) int { return strings.Compare(a.Class, b.Class) })
	return result, nil
}
