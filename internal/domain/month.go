package domain

import (
	"fmt"
	"strings"
	"time"
)

type Month struct {
	Year  int
	Month time.Month
}

func ParseMonth(s string) (Month, error) {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[4] != '-' {
		return Month{}, fmt.Errorf("month: expected YYYY-MM, got %q", s)
	}
	var y, m int
	if _, err := fmt.Sscanf(s, "%d-%d", &y, &m); err != nil {
		return Month{}, fmt.Errorf("month: expected YYYY-MM, got %q", s)
	}
	if y < 1 || y > 9999 || m < 1 || m > 12 {
		return Month{}, fmt.Errorf("month: out of range %q", s)
	}
	return Month{Year: y, Month: time.Month(m)}, nil
}

func MonthOf(t time.Time, loc *time.Location) Month {
	t = t.In(loc)
	return Month{Year: t.Year(), Month: t.Month()}
}

func (m Month) Key() string { return fmt.Sprintf("%04d-%02d", m.Year, int(m.Month)) }

func (m Month) String() string { return m.Key() }

func (m Month) AddMonths(n int) Month {
	total := m.Year*12 + (int(m.Month) - 1) + n
	return Month{Year: total / 12, Month: time.Month(total%12 + 1)}
}

func (m Month) Before(o Month) bool {
	return m.Year < o.Year || (m.Year == o.Year && m.Month < o.Month)
}

func (m Month) After(o Month) bool { return o.Before(m) }

func (m Month) Equal(o Month) bool { return m.Year == o.Year && m.Month == o.Month }

func (m Month) Start(loc *time.Location) time.Time {
	return time.Date(m.Year, m.Month, 1, 0, 0, 0, 0, loc)
}

func (m Month) Quarter() int { return (int(m.Month)-1)/3 + 1 }
