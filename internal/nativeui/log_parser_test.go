package nativeui

import "testing"

func TestParseLog4jXML(t *testing.T) {
	raw := []string{
		`<log4j:Event logger="net.minecraft.client.Minecraft" timestamp="1700000000000" level="INFO" thread="Render thread">`,
		`  <log4j:Message><![CDATA[Setting user: Steve]]></log4j:Message>`,
		`</log4j:Event>`,
		`<log4j:Event logger="net.minecraftforge.fml.ModLoader" timestamp="1700000001000" level="ERROR" thread="main">`,
		`  <log4j:Message><![CDATA[Failed to load &quot;mod&quot;`,
		`second line]]></log4j:Message>`,
		`  <log4j:Throwable><![CDATA[java.lang.RuntimeException: boom`,
		`	at a.b.C.d(C.java:1)`,
		`]]></log4j:Throwable>`,
		`</log4j:Event>`,
		`[13:08:31] [Render thread/WARN] [mixin/]: plain line`,
	}
	l := parseLogLines(raw, "")
	if len(l) != 6 {
		t.Fatalf("got %d lines: %+v", len(l), l)
	}
	if l[0].level != "INFO" || l[0].logger != "Minecraft" || l[0].thread != "Render thread" || l[0].msg != "Setting user: Steve" || l[0].time == "" {
		t.Fatalf("head: %+v", l[0])
	}
	if l[1].level != "ERROR" || l[1].kids != 3 || !l[1].stack {
		t.Fatalf("error head: %+v", l[1])
	}
	if !l[4].cont || l[4].parent != 1 || l[4].level != "ERROR" {
		t.Fatalf("stack line: %+v", l[4])
	}
	if l[5].level != "WARN" || l[5].cont {
		t.Fatalf("plain: %+v", l[5])
	}
}

func TestLogDeriveFilterSearchCollapse(t *testing.T) {
	lv := &logViewState{lines: parseLogLines([]string{
		"[1:00:00] [main/INFO] [a/]: hello",
		"[1:00:01] [main/ERROR] [b/]: crash",
		"java.lang.Exception: x",
		"\tat x.y(Z.java:1)",
		"[1:00:02] [main/WARN] [c/]: warn",
	}, "")}
	lv.derive()
	if len(lv.vis) != 3 || lv.counts["error"] != 1 || lv.counts["info"] != 1 {
		t.Fatalf("collapsed: %v %v", lv.vis, lv.counts)
	}
	lv.open = map[string]bool{lv.lines[1].raw: true}
	lv.derive()
	if len(lv.vis) != 5 {
		t.Fatalf("expanded: %v", lv.vis)
	}
	lv.levels = map[string]bool{"warn": true}
	lv.derive()
	if len(lv.vis) != 1 {
		t.Fatalf("warn only: %v", lv.vis)
	}
	lv.levels, lv.open, lv.query = nil, nil, "z.java"
	lv.derive()
	if len(lv.hits) != 1 || len(lv.vis) != 5 {
		t.Fatalf("search opens group: %v %v", lv.vis, lv.hits)
	}
	lv.query, lv.clearAt = "", 4
	lv.derive()
	if len(lv.vis) != 1 {
		t.Fatalf("clear: %v", lv.vis)
	}
}

func TestRepeatMerge(t *testing.T) {
	l := parseLogLines([]string{
		"[1:00:00] [main/WARN] [a/]: same",
		"[1:00:01] [main/WARN] [a/]: same",
		"[1:00:02] [main/WARN] [a/]: same",
		"[1:00:03] [main/WARN] [a/]: other",
	}, "")
	if len(l) != 2 || l[0].repeat != 3 || l[0].time != "1:00:02" {
		t.Fatalf("%+v", l)
	}
}
