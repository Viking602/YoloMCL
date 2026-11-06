package launcher

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWorldgenResourceComments(t *testing.T) {
	for _, raw := range []string{
		"{ // features\n\"effects\":{\"grass_color\":42} /* game resource */ }",
		"\ufeff{ # resource comment\n\"effects\":{\"grass_color\":42}}",
	} {
		snap := &WorldgenSnapshot{}
		collector := worldgenCollector{ctx: context.Background(), snapshot: snap, documents: map[string]WorldgenDocument{}}
		collector.accept("data/example/worldgen/biome/cave.json", "fixture", []byte(raw), nil)
		var v struct {
			Effects struct {
				GrassColor int `json:"grass_color"`
			}
		}
		if err := json.Unmarshal(collector.documents["biome/example:cave"].Data, &v); err != nil || v.Effects.GrassColor != 42 || len(snap.Issues) != 0 {
			t.Fatalf("game-compatible JSON rejected: %v %+v", err, snap.Issues)
		}
	}
	raw := []byte(`{"url":"https://example.test/#anchor","text":"escaped\" /*not a comment*/"}`)
	out, ok := worldgenJSONComments(raw)
	if !ok || string(out) != string(raw) {
		t.Fatal("comment tokens in strings were changed")
	}
	for _, raw := range []string{"{\"x\":1/*", "{\"x\":1/*comment*/2}", "{\"x\":1,}"} {
		out, ok := worldgenJSONComments([]byte(raw))
		if ok && boundedWorldgenJSON(out) {
			t.Fatalf("invalid JSON accepted: %s", raw)
		}
	}
}

func TestWorldgenArrayElisions(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`[1,]`, `[1,null]`},
		{`[,1,,2]`, `[null,1,null,2]`},
		{`[[1,],[],[,,]]`, `[[1,null],[],[null,null,null]]`},
		{`{"text":"[,]\\\"","features":[[],]}`, `{"text":"[,]\\\"","features":[[],null]}`},
	} {
		got := worldgenJSONArrayElisions([]byte(tc.raw))
		if string(got) != tc.want || !boundedWorldgenJSON(got) {
			t.Fatalf("%s became %s, want %s", tc.raw, got, tc.want)
		}
	}
	for _, invalid := range []string{`{"x":1,}`, `[{"x":1,},]`, strings.Repeat("[", 65) + "," + strings.Repeat("]", 65), "[" + strings.Repeat(",", maxWorldgenJSON/2) + "]"} {
		if boundedWorldgenJSON(worldgenJSONArrayElisions([]byte(invalid))) {
			t.Fatal("malformed or oversized JSON accepted")
		}
	}
}
