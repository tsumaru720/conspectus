package domain

import "testing"

func TestParseMonth(t *testing.T) {
	cases := []struct {
		in   string
		want Month
		ok   bool
	}{
		{"2026-08", Month{2026, 8}, true},
		{" 2026-08 ", Month{2026, 8}, true},
		{"0001-01", Month{1, 1}, true},
		{"9999-12", Month{9999, 12}, true},
		{"", Month{}, false},
		{"abc", Month{}, false},
		{"2026-8", Month{}, false},
		{"2026", Month{}, false},
		{"2026/08", Month{}, false},
		{"2026-00", Month{}, false},
		{"2026-13", Month{}, false},
		{"0000-05", Month{}, false},
		{"2026-08-01", Month{}, false},
	}
	for _, c := range cases {
		got, err := ParseMonth(c.in)
		if c.ok && err != nil {
			t.Errorf("ParseMonth(%q) unexpected error: %v", c.in, err)
			continue
		}
		if !c.ok && err == nil {
			t.Errorf("ParseMonth(%q) = %s, want error", c.in, got)
			continue
		}
		if c.ok && got != c.want {
			t.Errorf("ParseMonth(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}
