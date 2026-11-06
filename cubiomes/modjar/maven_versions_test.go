package modjar

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestMavenIndependentVersionOracle(t *testing.T) {
	data, err := os.ReadFile("testdata/maven-versions-3.8.5.tsv")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatal("invalid oracle line")
		}
		want, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		if got := cmp.Compare(compareMaven(mavenVersion(fields[0]), mavenVersion(fields[1])), 0); got != want {
			t.Fatalf("%s vs %s: %d want %d", fields[0], fields[1], got, want)
		}
		count++
	}
	if count != 1600 {
		t.Fatal("incomplete oracle", count)
	}
}

func TestMavenRangesAndContainedSelection(t *testing.T) {
	for _, tt := range []struct {
		r, v string
		want bool
	}{
		{"[0.0.13.42,)", "0.0.4.15", false}, {"[0.0.13.42,)", "0.0.13.42", true},
		{"(,1.0],[1.2,)", "1.1", false}, {"(,1.0],[1.2,)", "1.2", true},
		{"[1.0]", "1-final", true}, {"(1.0,2.0)", "1.0", false}, {"(1.0,2.0)", "2.0-rc1", true},
	} {
		r, err := parseMavenRange(tt.r)
		if err != nil {
			t.Fatal(err)
		}
		if r.contains(mavenVersion(tt.v)) != tt.want {
			t.Fatalf("%s %s", tt.r, tt.v)
		}
	}
	root := func(name, version, requested string, payload byte) Archive {
		j := containedJar{Path: "META-INF/jarjar/dependency.jar"}
		j.Identifier.Group = "example"
		j.Identifier.Artifact = "shared"
		j.Version.ArtifactVersion = version
		j.Version.Range = requested
		metadata, err := json.Marshal(struct {
			Jars []containedJar `json:"jars"`
		}{[]containedJar{j}})
		if err != nil {
			t.Fatal(err)
		}
		return Archive{name, fixtureArchive(t, map[string][]byte{"META-INF/jarjar/metadata.json": metadata, j.Path: nestedArchiveBytes(t, []byte{payload}, false)})}
	}
	for _, loader := range []string{"forge", "neoforge"} {
		cp, err := NewClasspath(context.Background(), []Archive{root("first", "0.0.4.15", "[0.0.4,)", 1), root("second", "0.0.13.42", "[0.0.13.42,)", 2)}, loader)
		if err != nil {
			t.Fatal(err)
		}
		got, err := cp.Class("example/Dependency")
		if err != nil || len(got) != 1 || got[0] != 2 {
			t.Fatalf("%s selection: %v %v", loader, got, err)
		}
		if len(cp.Archives()) != 3 {
			t.Fatal("excluded library remained active")
		}
		cp.Close()
	}
	cp, err := NewClasspath(context.Background(), []Archive{root("first", "1", "[1,)", 1), root("second", "2", "[1,)", 2)}, "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	// Several versions satisfy every range: the loader keeps the highest.
	if got, err := cp.Class("example/Dependency"); err != nil || len(got) != 1 || got[0] != 2 {
		t.Fatal(fmt.Sprint("compatible versions should select the highest: ", got, err))
	}
	sole, err := NewClasspath(context.Background(), []Archive{root("sole", "1", "[2,)", 1)}, "forge")
	if err != nil {
		t.Fatal(err)
	}
	defer sole.Close()
	if _, err := sole.Class("example/Dependency"); err == nil || !strings.Contains(err.Error(), "0 eligible") {
		t.Fatal("a single artifact must still satisfy its declared range", err)
	}
}
