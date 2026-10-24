package nativeui

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// logLine is one parsed log line. Continuation lines (stack traces, the
// rest of a multi-line message) keep their parent's level; Parent is the
// index of the line they belong to, -1 for a head line.
type logLine struct {
	Raw, Time, Full, Thread, Level, Logger, Msg string
	Repeat                                      int // identical lines merged into this one (×N)
	Cont                                        bool
	Parent                                      int
	Kids                                        int  // continuation lines under a head
	Stack                                       bool // kids include a stack trace
}

func (l logLine) Structured() bool {
	return l.Time != "" || l.Thread != "" || l.Logger != ""
}

var (
	logRE      = regexp.MustCompile(`^\[([^\]]+)\] \[([^\]]*?)/([A-Z]+)\](?: \[([^\]]*)\])?:? ?(.*)$`)
	excRE      = regexp.MustCompile(`^(?:[A-Za-z_$][\w$]*\.)+[\w$]*(?:Exception|Error|Throwable)\b`)
	log4jEvent = regexp.MustCompile(`([a-z]+)="([^"]*)"`)
)

const (
	cdataMsg = "<log4j:Message><![CDATA["
	cdataThr = "<log4j:Throwable><![CDATA["
)

func shortLogger(l string) string {
	l = strings.TrimSuffix(l, "/")
	for _, p := range []string{"net.minecraftforge.", "net.neoforged.", "net.minecraft.", "net.fabricmc.", "com.mojang."} {
		l = strings.TrimPrefix(l, p)
	}
	if i := strings.LastIndex(l, "."); i >= 0 && i < len(l)-1 {
		l = l[i+1:]
	}
	return l
}

// logParser decodes one line at a time, so memory stays bounded by the
// lines kept; it understands plain Forge/vanilla lines and the log4j XML
// events Forge writes to stdout.
type logParser struct {
	out   []logLine
	level string
	head  int // index of the current head line, -1 none
	ev    *logLine
	cdata string // "" | "msg" | "thr": inside a multi-line CDATA block
}

func (p *logParser) push(l logLine) {
	// An identical line right after a plain one merges into it as ×N.
	if !l.Cont && p.head >= 0 && p.head == len(p.out)-1 {
		h := &p.out[p.head]
		if h.Kids == 0 && h.Msg == l.Msg && h.Level == l.Level && h.Logger == l.Logger && h.Thread == l.Thread && l.Msg != "" {
			if h.Repeat == 0 {
				h.Repeat = 1
			}
			h.Repeat++
			h.Time, h.Full = l.Time, l.Full
			return
		}
	}
	if l.Cont && p.head >= 0 {
		l.Parent = p.head
		h := &p.out[p.head]
		h.Kids++
		t := strings.TrimSpace(l.Msg)
		if strings.HasPrefix(t, "at ") || strings.HasPrefix(t, "Caused by") {
			h.Stack = true
		}
	} else {
		l.Cont, l.Parent = false, -1
		p.head = len(p.out)
	}
	p.out = append(p.out, l)
}

// eventHead starts an XML event's line, normalized for copying.
func (p *logParser) eventHead(msg string) {
	e := *p.ev
	e.Msg = msg
	e.Raw = "[" + e.Time + "] [" + e.Thread + "/" + e.Level + "] [" + e.Logger + "]: " + msg
	e.Logger = shortLogger(e.Logger)
	p.push(e)
}

func (p *logParser) contLine(text string) {
	p.push(logLine{Raw: text, Level: p.level, Msg: strings.TrimLeft(text, " \t"), Cont: true})
}

func (p *logParser) line(r string) {
	t := strings.TrimSpace(r)
	if p.cdata != "" {
		body, done := r, false
		if i := strings.Index(r, "]]>"); i >= 0 {
			body, done = r[:i], true
		}
		if body != "" || !done {
			p.contLine(body)
		}
		if done {
			p.cdata = ""
		}
		return
	}
	switch {
	case strings.HasPrefix(t, "<log4j:Event"):
		ev := logLine{}
		for _, m := range log4jEvent.FindAllStringSubmatch(t, -1) {
			v := html.UnescapeString(m[2])
			switch m[1] {
			case "logger":
				ev.Logger = v
			case "level":
				ev.Level = v
			case "thread":
				ev.Thread = v
			case "timestamp":
				if ms, e := strconv.ParseInt(v, 10, 64); e == nil {
					t := time.UnixMilli(ms)
					ev.Time, ev.Full = t.Format("15:04:05"), t.Format("2006-01-02 15:04:05.000")
				}
			}
		}
		p.ev, p.level = &ev, ev.Level
		return
	case strings.HasPrefix(t, cdataMsg) || strings.HasPrefix(t, cdataThr):
		thr := strings.HasPrefix(t, cdataThr)
		body := strings.TrimPrefix(strings.TrimPrefix(t, cdataMsg), cdataThr)
		open := true
		if i := strings.Index(body, "]]>"); i >= 0 {
			body, open = body[:i], false
		}
		if open {
			p.cdata = "msg"
		}
		if !thr && p.ev != nil {
			p.eventHead(body)
			p.ev = nil
		} else if body != "" {
			p.contLine(body)
		}
		return
	case strings.HasPrefix(t, "</log4j:") || strings.HasPrefix(t, "<log4j:Properties") || strings.HasPrefix(t, "<log4j:Data") || strings.HasPrefix(t, "<log4j:Locat"):
		return
	}
	if m := logRE.FindStringSubmatch(r); m != nil {
		tm, full := m[1], m[1]
		if i := strings.LastIndex(tm, " "); i >= 0 {
			tm = tm[i+1:]
		}
		p.level = m[3]
		p.push(logLine{Raw: r, Time: tm, Full: full, Thread: m[2], Level: m[3], Logger: shortLogger(m[4]), Msg: m[5]})
		return
	}
	cont := t != r || strings.HasPrefix(t, "at ") || strings.HasPrefix(t, "Caused by") || strings.HasPrefix(t, "...") || excRE.MatchString(t)
	// A crash report's exception is a heading, not a continuation of the
	// preceding blank line. Keep the cause visible when its frames collapse.
	if excRE.MatchString(t) && (p.head < 0 || !p.out[p.head].Structured()) {
		cont = false
	}
	p.push(logLine{Raw: r, Level: p.level, Msg: t, Cont: cont})
}

func parseLogLines(raw []string, prev string) []logLine {
	p := logParser{out: make([]logLine, 0, len(raw)), level: prev, head: -1}
	for _, r := range raw {
		p.line(r)
	}
	return p.out
}

func levelRank(l string) int {
	switch l {
	case "WARN":
		return 1
	case "ERROR", "FATAL":
		return 2
	}
	return 0
}

// levelKey groups levels into the filter chips.
func levelKey(l string) string {
	switch l {
	case "WARN":
		return "warn"
	case "ERROR", "FATAL":
		return "error"
	case "DEBUG", "TRACE":
		return "debug"
	}
	return "info"
}
