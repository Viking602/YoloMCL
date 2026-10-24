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
	if l[0].Level != "INFO" || l[0].Logger != "Minecraft" || l[0].Thread != "Render thread" || l[0].Msg != "Setting user: Steve" || l[0].Time == "" {
		t.Fatalf("head: %+v", l[0])
	}
	if l[1].Level != "ERROR" || l[1].Kids != 3 || !l[1].Stack {
		t.Fatalf("error head: %+v", l[1])
	}
	if !l[4].Cont || l[4].Parent != 1 || l[4].Level != "ERROR" {
		t.Fatalf("stack line: %+v", l[4])
	}
	if l[5].Level != "WARN" || l[5].Cont {
		t.Fatalf("plain: %+v", l[5])
	}
}

func TestRepeatMerge(t *testing.T) {
	l := parseLogLines([]string{
		"[1:00:00] [main/WARN] [a/]: same",
		"[1:00:01] [main/WARN] [a/]: same",
		"[1:00:02] [main/WARN] [a/]: same",
		"[1:00:03] [main/WARN] [a/]: other",
	}, "")
	if len(l) != 2 || l[0].Repeat != 3 || l[0].Time != "1:00:02" {
		t.Fatalf("%+v", l)
	}
}
