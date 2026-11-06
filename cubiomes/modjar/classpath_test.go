package modjar

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestDependencyClassesParticipateInRegistrationAndHash(t *testing.T) {
	files := archiveFiles(t, loaderFixture(t, "fabric", true))
	region := files["example/Region.class"]
	delete(files, "example/Region.class")
	z := fixtureArchive(t, files)
	reads := 0
	options := Options{Minecraft: "1.21.1", Loader: "fabric", ModID: "sample", Class: func(name string) ([]byte, error) {
		if name == "example/Region" {
			reads++
			return region, nil
		}
		return nil, nil
	}}
	first, err := ReadTerraBlender(context.Background(), z, options)
	if err != nil || len(first.Regions) != 1 || reads != 1 {
		t.Fatalf("dependency result %+v reads=%d %v", first, reads, err)
	}
	whole, err := ReadTerraBlender(context.Background(), loaderFixture(t, "fabric", true), Options{Minecraft: "1.21.1", Loader: "fabric", ModID: "sample"})
	if err != nil || first.ProgramSHA256 != whole.ProgramSHA256 {
		t.Fatalf("classpath provenance/hash mismatch: %v", err)
	}
	region = relocateFixture(region, strings.NewReplacer("sample", "changed"))
	changed, err := ReadTerraBlender(context.Background(), z, options)
	if err != nil || changed.ProgramSHA256 == first.ProgramSHA256 || changed.Regions[0].Parameters[0].Biome != "changed:biome" {
		t.Fatalf("dependency did not invalidate input: %+v %v", changed, err)
	}
	options.Class = func(string) ([]byte, error) { return nil, errors.New("dependency unavailable") }
	result, err := ReadTerraBlender(context.Background(), z, options)
	if err == nil || len(result.Regions) != 0 {
		t.Fatal("dependency failure produced a partial result")
	}
}

func TestUnrelatedClassesDoNotConsumeExecutionBudget(t *testing.T) {
	for _, loader := range []string{"forge", "fabric"} {
		t.Run(loader, func(t *testing.T) {
			options := Options{Minecraft: "1.21.1", Loader: loader, ModID: "sample"}
			base := loaderFixture(t, loader, true)
			want, err := ReadTerraBlender(context.Background(), base, options)
			if err != nil {
				t.Fatal(err)
			}
			files := archiveFiles(t, base)
			for i := range 18 {
				name := fmt.Sprintf("unrelated/Large%d", i)
				c := &fixtureClass{}
				attr := words(c.utf("UnusedData"))
				attr = binary.BigEndian.AppendUint32(attr, 1<<20)
				c.attrs = append(c.attrs, append(attr, make([]byte, 1<<20)...))
				files[name+".class"] = c.bytes(name, "java/lang/Object")
			}
			got, err := ReadTerraBlender(context.Background(), fixtureArchive(t, files), options)
			if err != nil || got.ProgramSHA256 != want.ProgramSHA256 || len(got.Regions) != 1 {
				t.Fatalf("unrelated payload changed registration: %+v %v", got, err)
			}
		})
	}
}

func TestCapturedRegionUsesVirtualIdentityTypeAndWeight(t *testing.T) {
	z := regionFixtureOverrides(t, "sample", true, false, func(c *fixtureClass) {
		c.method("getWeight", "()I", 1, []byte{16, 37, 172})
		code := c.instruction(178, "terrablender/api/RegionType", "NETHER", "Lterrablender/api/RegionType;")
		c.method("getType", "()Lterrablender/api/RegionType;", 1, append(code, 176))
		code = c.string("sample:overridden")
		code = append(code, c.instruction(184, "net/minecraft/resources/ResourceLocation", "parse", "(Ljava/lang/String;)Lnet/minecraft/resources/ResourceLocation;")...)
		c.method("getName", "()Lnet/minecraft/resources/ResourceLocation;", 1, append(code, 176))
	})
	got, err := ReadTerraBlender(context.Background(), z, Options{Minecraft: "1.21.1"})
	if err != nil || len(got.Regions) != 1 {
		t.Fatalf("capture: %+v %v", got, err)
	}
	r := got.Regions[0]
	if r.ID != "sample:overridden" || r.Dimension != cubiomes.Nether || r.Weight != 37 {
		t.Fatalf("override ignored: %+v", r)
	}
}

func TestVirtualOverridePrecedesNativeFrameworkAPI(t *testing.T) {
	c := &fixtureClass{}
	c.method("getWeight", "()I", 1, []byte{16, 37, 172})
	v := newMachine(context.Background(), map[string][]byte{"example/CustomRegion": c.bytes("example/CustomRegion", "terrablender/api/Region")})
	h := &bindings{vm: v}
	v.host = h
	o := v.alloc("example/CustomRegion", &region{weight: 3})
	got := v.method(reference{"terrablender/api/Region", "getWeight", "()I"}, o, true, nil)
	if got != int32(37) {
		t.Fatalf("override was bypassed: %v", got)
	}
}

func TestLocalClassDoesNotHideClasspathConflict(t *testing.T) {
	z := loaderFixture(t, "fabric", true)
	files := archiveFiles(t, z)
	main := files["example/Main.class"]
	options := Options{Minecraft: "1.21.1", Loader: "fabric", ModID: "sample", Class: func(name string) ([]byte, error) {
		if name == "example/Main" {
			return main, nil
		}
		return nil, nil
	}}
	if _, err := ReadTerraBlender(context.Background(), z, options); err != nil {
		t.Fatal(err)
	}
	main = relocateFixture(main, strings.NewReplacer("example/Registration", "other/Registration"))
	got, err := ReadTerraBlender(context.Background(), z, options)
	if err == nil || !strings.Contains(err.Error(), "conflicting local/classpath") || len(got.Regions) != 0 {
		t.Fatalf("local definition silently shadowed a dependency: %+v %v", got, err)
	}
}
