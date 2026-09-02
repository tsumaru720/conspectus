package app

import (
	"bytes"
	"io"
	"strconv"
)

// ANSI colour codes; 256-colour for orange and purple, which the base
// palette lacks. Bright variants for the standard colours read better on
// dark terminal backgrounds.
const (
	colGreen   = "92"
	colCyan    = "96"
	colRed     = "91"
	colBlue    = "94"
	colMagenta = "95"
	colOrange  = "38;5;208"
	colPurple  = "38;5;135"
	colReset   = "\x1b[0m"
)

// colourWriter colours the level, method and status tokens of text-format
// slog lines on their way to the terminal; time, bytes and duration stay
// plain. Only used for text logs - json output is left machine-readable.
type colourWriter struct {
	w io.Writer

	pending []byte
}

func (c *colourWriter) Write(p []byte) (int, error) {
	data := append(c.pending, p...)
	c.pending = c.pending[:0]
	var out []byte
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			c.pending = append(c.pending, data...)
			break
		}
		out = colourLine(out, data[:i])
		out = append(out, '\n')
		data = data[i+1:]
	}
	_, err := c.w.Write(out)
	return len(p), err
}

// colourLine re-emits one log line, wrapping recognised tokens in the
// matching ANSI colour. Tokens are key=value pairs separated by single
// spaces, where a value containing spaces or special characters is
// double-quoted.
func colourLine(dst, line []byte) []byte {
	for i, tok := range splitFields(line) {
		if i > 0 {
			dst = append(dst, ' ')
		}
		key, val, _ := bytes.Cut(tok, []byte{'='})
		var code string
		switch string(key) {
		case "level":
			code = levelColour(string(val))
		case "method":
			code = methodColour(string(val))
		case "status":
			code = statusColour(string(val))
		}
		if code == "" {
			dst = append(dst, tok...)
			continue
		}
		dst = append(dst, "\x1b["...)
		dst = append(dst, code...)
		dst = append(dst, 'm')
		dst = append(dst, tok...)
		dst = append(dst, colReset...)
	}
	return dst
}

// splitFields splits a line on unquoted spaces, keeping quoted runs
// (including escaped quotes) together as one field.
func splitFields(line []byte) [][]byte {
	var toks [][]byte
	start := 0
	inQuote, escaped := false, false
	for i, b := range line {
		switch {
		case escaped:
			escaped = false
		case inQuote && b == '\\':
			escaped = true
		case b == '"':
			inQuote = !inQuote
		case b == ' ' && !inQuote:
			toks = append(toks, line[start:i])
			start = i + 1
		}
	}
	if start < len(line) {
		toks = append(toks, line[start:])
	}
	return toks
}

func levelColour(level string) string {
	switch level {
	case "DEBUG":
		return colPurple
	case "INFO":
		return colGreen
	case "WARN":
		return colOrange
	case "ERROR":
		return colRed
	}
	return ""
}

func methodColour(method string) string {
	switch method {
	case "GET":
		return colCyan
	case "POST":
		return colGreen
	case "PUT":
		return colBlue
	case "PATCH":
		return colMagenta
	case "DELETE":
		return colRed
	}
	return ""
}

func statusColour(status string) string {
	n, err := strconv.Atoi(string(status))
	if err != nil || n < 100 {
		return ""
	}
	switch {
	case n < 300:
		return colGreen
	case n < 400:
		return colCyan
	case n < 500:
		return colOrange
	default:
		return colRed
	}
}
