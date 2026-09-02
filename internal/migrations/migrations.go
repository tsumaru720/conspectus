package migrations

import (
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

type Migration struct {
	Version  int
	Name     string
	Source   string
	Contents string
	Raw      bool
}

var filePattern = regexp.MustCompile(`^([0-9]{4})_([a-z0-9_]+)\.sql$`)

func Load(sources []fs.FS, names []string) ([]Migration, error) {
	byVersion := map[int]Migration{}
	for i, src := range sources {
		label := "embedded"
		if i < len(names) && names[i] != "" {
			label = names[i]
		}
		entries, err := fs.ReadDir(src, ".")
		if err != nil {
			return nil, fmt.Errorf("migrations: read dir %s: %w", label, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			m := filePattern.FindStringSubmatch(e.Name())
			if m == nil {
				continue
			}
			v, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			data, err := fs.ReadFile(src, e.Name())
			if err != nil {
				return nil, fmt.Errorf("migrations: read %s/%s: %w", label, e.Name(), err)
			}
			contents := string(data)
			byVersion[v] = Migration{
				Version:  v,
				Name:     e.Name(),
				Source:   label,
				Contents: contents,
				Raw:      isRawDirective(contents),
			}
		}
	}
	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		out = append(out, m)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Version < out[j-1].Version; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	for i, m := range out {
		if m.Version != i+1 {
			return nil, fmt.Errorf("migrations: version sequence must start at 0001 and be gapless; found %04d at position %d", m.Version, i+1)
		}
	}
	return out, nil
}

var rawDirective = regexp.MustCompile(`(?m)^--\s*\+conspectus:raw\s*$`)

func isRawDirective(contents string) bool {
	return rawDirective.MatchString(strings.TrimSpace(contents))
}

func Statements(src string) []string {
	var (
		stmts []string
		cur   strings.Builder
	)

	flush := func() {
		s := strings.TrimSpace(cur.String())
		cur.Reset()
		if s == "" {
			return
		}
		if strings.TrimSpace(stripComments(s)) == "" {
			return
		}
		stmts = append(stmts, s)
	}

	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '-' && i+1 < n && src[i+1] == '-':
			for i < n && src[i] != '\n' {
				cur.WriteByte(src[i])
				i++
			}
		case c == '#':
			for i < n && src[i] != '\n' {
				cur.WriteByte(src[i])
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				cur.WriteString(src[i:])
				i = n
			} else {
				cur.WriteString(src[i : i+2+end+2])
				i += 2 + end + 2
			}
		case c == '\'' || c == '"' || c == '`':
			quote := c
			cur.WriteByte(c)
			i++
			for i < n {
				cur.WriteByte(src[i])
				if src[i] == '\\' && quote != '`' && i+1 < n {
					cur.WriteByte(src[i+1])
					i += 2
					continue
				}
				if src[i] == quote {
					if i+1 < n && src[i+1] == quote {
						cur.WriteByte(src[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '$' && i+1 < n && src[i+1] == '$':
			cur.WriteString("$$")
			i += 2
			end := strings.Index(src[i:], "$$")
			if end < 0 {
				cur.WriteString(src[i:])
				i = n
			} else {
				cur.WriteString(src[i : i+end+2])
				i += end + 2
			}
		case c == ';':
			flush()
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return stmts
}

func stripComments(src string) string {
	var b strings.Builder
	i, n := 0, len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '-' && i+1 < n && src[i+1] == '-':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				i = n
			} else {
				i += 2 + end + 2
			}
		case c == '\'' || c == '"' || c == '`':
			quote := c
			b.WriteByte(c)
			i++
			for i < n {
				b.WriteByte(src[i])
				if src[i] == '\\' && quote != '`' && i+1 < n {
					if i+1 < n {
						b.WriteByte(src[i+1])
					}
					i += 2
					continue
				}
				if src[i] == quote {
					if i+1 < n && src[i+1] == quote {
						if i+1 < n {
							b.WriteByte(src[i+1])
						}
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
