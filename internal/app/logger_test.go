package app

import (
	"bytes"
	"strings"
	"testing"
)

const esc = "\x1b["

func TestColourWriterColoursLevelMethodStatus(t *testing.T) {
	var buf bytes.Buffer
	w := &colourWriter{w: &buf}
	line := "time=2026-09-02T10:00:00.000Z level=INFO msg=http method=POST status=404 " +
		"path=\"/import?x=1 y=2\" bytes=10 dur=1ms\n"
	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, esc+"92mlevel=INFO"+esc+"0m") {
		t.Errorf("INFO level not green: %q", out)
	}
	if !strings.Contains(out, esc+"92mmethod=POST"+esc+"0m") {
		t.Errorf("POST method not green: %q", out)
	}
	if !strings.Contains(out, esc+"38;5;208mstatus=404"+esc+"0m") {
		t.Errorf("404 status not orange: %q", out)
	}
	if !strings.Contains(out, `path="/import?x=1 y=2"`) {
		t.Errorf("quoted path mangled: %q", out)
	}
	if !strings.HasPrefix(out, "time=2026") {
		t.Errorf("time token coloured or reordered: %q", out)
	}
	if !strings.Contains(out, "bytes=10 dur=1ms") {
		t.Errorf("bytes/dur tokens altered: %q", out)
	}
}

func TestColourWriterLevelAndStatusClasses(t *testing.T) {
	cases := []struct {
		token string
		code  string
	}{
		{"level=DEBUG", "38;5;135"},
		{"level=WARN", "38;5;208"},
		{"level=ERROR", "91"},
		{"method=GET", "96"},
		{"method=DELETE", "91"},
		{"method=PATCH", "95"},
		{"method=PUT", "94"},
		{"status=200", "92"},
		{"status=302", "96"},
		{"status=500", "91"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		w := &colourWriter{w: &buf}
		if _, err := w.Write([]byte("msg=http " + tc.token + "\n")); err != nil {
			t.Fatal(err)
		}
		want := esc + tc.code + "m" + tc.token + esc + "0m"
		if !strings.Contains(buf.String(), want) {
			t.Errorf("%s: want %q in %q", tc.token, want, buf.String())
		}
	}
}

func TestColourWriterUncolouredTokens(t *testing.T) {
	var buf bytes.Buffer
	w := &colourWriter{w: &buf}
	if _, err := w.Write([]byte("method=OPTIONS status=99 msg=\"storage open\" driver=mysql\n")); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if out != "method=OPTIONS status=99 msg=\"storage open\" driver=mysql\n" {
		t.Errorf("unrecognised tokens must pass through untouched: %q", out)
	}
}

func TestColourWriterQuotesAndEscapes(t *testing.T) {
	var buf bytes.Buffer
	w := &colourWriter{w: &buf}
	// A quoted message with escaped quotes and spaces must stay one field.
	if _, err := w.Write([]byte("level=WARN msg=\"say \\\"hello world\\\" now\"\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `msg="say \"hello world\" now"`) {
		t.Errorf("quoted value split: %q", buf.String())
	}
}

func TestColourWriterSplitWrites(t *testing.T) {
	var buf bytes.Buffer
	w := &colourWriter{w: &buf}
	half := len("level=INFO msg=http\n") / 2
	line := "level=INFO msg=http\n"
	if _, err := w.Write([]byte(line[:half])); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(line[half:])); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), esc+"92mlevel=INFO"+esc+"0m") {
		t.Errorf("split write not reassembled: %q", buf.String())
	}
}

func TestColourWriterMethodSplitAcrossChunks(t *testing.T) {
	var buf bytes.Buffer
	w := &colourWriter{w: &buf}
	if _, err := w.Write([]byte("msg=http method=GE")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("T status=200\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), esc+"96mmethod=GET"+esc+"0m") {
		t.Errorf("token split across writes not coloured: %q", buf.String())
	}
}
