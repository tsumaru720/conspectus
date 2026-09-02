package domain

import "testing"

func TestParseMoneyAcceptsFormattedInput(t *testing.T) {
	cases := []struct {
		in   string
		want Money
		ok   bool
	}{
		{"2500", 250000, true},
		{"2500.00", 250000, true},
		{"£1,342.00", 134200, true},
		{"1,342.00", 134200, true},
		{"1,342", 134200, true},
		{"$ 1500.50", 150050, true},
		{"€ 2 500", 250000, true},
		{"-45.99", -4599, true},
		{"-£45.99", -4599, true},
		{" 12,345.67 ", 1234567, true},
		{"", 0, false},
		{"£", 0, false},
		{"1,34", 13400, true},
		{"1.234", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, err := ParseMoney(c.in)
		if c.ok && err != nil {
			t.Errorf("ParseMoney(%q) unexpected error: %v", c.in, err)
			continue
		}
		if !c.ok && err == nil {
			t.Errorf("ParseMoney(%q) = %d, want error", c.in, got)
			continue
		}
		if c.ok && got != c.want {
			t.Errorf("ParseMoney(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
