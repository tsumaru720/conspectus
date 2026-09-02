package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Money int64

const MaxMoney Money = 9999999999999

const MinMoney Money = -9999999999999

var ErrMoneyFormat = errors.New("money: expected decimal with at most 2 dp, e.g. \"2506.00\"")

var ErrMoneyRange = errors.New("money: value out of range for DECIMAL(13,2)")

func ParseMoney(s string) (Money, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ',', ' ', '\t', '£', '$', '€', '¥', 0x00A0:
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		return 0, ErrMoneyFormat
	}
	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	if s == "" || s == "." {
		return 0, ErrMoneyFormat
	}
	dot := strings.IndexByte(s, '.')
	intPart, fracPart := s, ""
	if dot >= 0 {
		intPart, fracPart = s[:dot], s[dot+1:]
		if len(fracPart) > 2 {
			return 0, fmt.Errorf("%w: more than 2 decimal places in %q", ErrMoneyFormat, s)
		}
	}
	if intPart == "" {
		intPart = "0"
	}
	for _, ch := range intPart {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("%w: %q", ErrMoneyFormat, s)
		}
	}
	for _, ch := range fracPart {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("%w: %q", ErrMoneyFormat, s)
		}
	}
	var units int64
	for _, ch := range intPart {
		d := int64(ch - '0')
		units = units*10 + d
		if units > int64(MaxMoney) {
			return 0, ErrMoneyRange
		}
	}
	var frac int64
	switch len(fracPart) {
	case 1:
		frac = int64(fracPart[0]-'0') * 10
	case 2:
		frac = int64(fracPart[0]-'0')*10 + int64(fracPart[1]-'0')
	}
	m := Money(units*100 + frac)
	if neg {
		m = -m
	}
	if m > MaxMoney || m < MinMoney {
		return 0, ErrMoneyRange
	}
	return m, nil
}

func (m Money) Format() string {
	neg := m < 0
	if neg {
		m = -m
	}
	return fmt.Sprintf("%s%d.%02d", signPrefix(neg), int64(m)/100, int64(m)%100)
}

func (m Money) String() string { return m.Format() }

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.Format()) }

func (m *Money) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "null" {
		*m = 0
		return nil
	}
	if len(s) >= 2 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		v, err := ParseMoney(str)
		if err != nil {
			return err
		}
		*m = v
		return nil
	}
	v, err := ParseMoney(json.Number(s).String())
	if err != nil {
		return fmt.Errorf("%w (money must be a decimal string like \"2506.00\")", err)
	}
	*m = v
	return nil
}

func (m Money) Float() float64 { return float64(m) / 100 }

func signPrefix(neg bool) string {
	if neg {
		return "-"
	}
	return ""
}
