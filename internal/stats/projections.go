package stats

import (
	"math"
	"math/rand"
	"sort"
	"strconv"

	"conspectus/internal/domain"
)

type Projections struct {
	HorizonMonths int
	FromMonth     domain.Month
	Months        []string
	Expected      []float64
	Upper         []float64
	Lower         []float64
	CAGRPath      []float64
	Milestones    []Milestone
	TargetCurve   TargetCurve
	Assumptions   map[string]any
}

type Milestone struct {
	Target  float64
	Months  int
	Date    string
	Reached bool
	Already bool
}

type TargetCurve struct {
	Value           float64
	Year            int
	MonthsRemaining int
	RequiredMonthly float64
	RequiredCAGR    float64
	CurveMonths     []string
	CurveValues     []float64
}

type ProjectionParams struct {
	Horizon    int
	Window     int
	DepositWin string
	Targets    []float64
	CurveValue float64
	CurveYear  int
}

func Project(in Input, p ProjectionParams) Projections {
	out := Projections{HorizonMonths: p.Horizon, Assumptions: map[string]any{}}
	if p.Horizon <= 0 {
		p.Horizon = 120
		out.HorizonMonths = 120
	}

	s := BuildSeries(in)
	visible := s.Visible()
	if len(visible) == 0 {
		return out
	}
	last := visible[len(visible)-1]
	out.FromMonth = last.Month

	c := Chain(s.ChainPoints(), s.HasBaseline)
	rets := c.monthlyReturnsAdj()
	if p.Window > 0 && len(rets) > p.Window {
		rets = rets[len(rets)-p.Window:]
	}
	winLabel := "all"
	if p.Window > 0 {
		winLabel = strconv.Itoa(p.Window)
	}

	mu, sigma := meanStd(rets)
	v0 := last.V.Float()

	var deposits []MonthPoint
	switch p.DepositWin {
	case "zero":
		deposits = nil
	case "trailing60":
		if n := len(visible); n > 60 {
			deposits = visible[n-60:]
		} else {
			deposits = visible
		}
	case "all":
		deposits = visible
	default:
		if n := len(visible); n > 12 {
			deposits = visible[n-12:]
		} else {
			deposits = visible
		}
	}
	delta := 0.0
	if len(deposits) > 0 {
		var sum domain.Money
		for _, pt := range deposits {
			sum += pt.F
		}
		delta = sum.Float() / float64(len(deposits))
	}

	out.Assumptions["mu_monthly"] = mu
	out.Assumptions["sigma_monthly"] = sigma
	out.Assumptions["deposit_monthly"] = delta
	out.Assumptions["window"] = winLabel
	out.Assumptions["deposit_window"] = p.DepositWin
	out.Assumptions["value_now"] = v0
	out.Assumptions["months_history"] = len(rets)

	path := func(growth float64) []float64 {
		vals := make([]float64, p.Horizon)
		v := v0
		for k := 0; k < p.Horizon; k++ {
			v = (v + delta) * (1 + growth)
			vals[k] = v
		}
		return vals
	}

	prod := 1.0
	for _, r := range rets {
		prod *= 1 + r
	}
	cagr := 0.0
	if len(rets) > 0 {
		cagr = math.Pow(prod, 1.0/float64(len(rets))) - 1
	}
	out.Assumptions["cagr_monthly"] = cagr
	out.CAGRPath = path(cagr)

	const simPaths = 500
	out.Assumptions["simulations"] = simPaths
	out.Assumptions["band_pct"] = "10/90"
	if len(rets) > 0 {
		rng := rand.New(rand.NewSource(42))
		grid := make([][]float64, p.Horizon)
		for range simPaths {
			v := v0
			for k := 0; k < p.Horizon; k++ {
				v = (v + delta) * (1 + rets[rng.Intn(len(rets))])
				grid[k] = append(grid[k], v)
			}
		}
		out.Expected = make([]float64, p.Horizon)
		out.Upper = make([]float64, p.Horizon)
		out.Lower = make([]float64, p.Horizon)
		for k := 0; k < p.Horizon; k++ {
			row := grid[k]
			sort.Float64s(row)
			out.Lower[k] = row[pctIndex(0.10, len(row))]
			out.Expected[k] = row[pctIndex(0.50, len(row))]
			out.Upper[k] = row[pctIndex(0.90, len(row))]
		}
	} else {
		out.Expected = path(mu)
		out.Upper = path(mu + sigma)
		out.Lower = path(mu - sigma)
	}
	for i := range out.Lower {
		if out.Lower[i] < 0 {
			out.Lower[i] = 0
		}
	}

	m := last.Month
	for k := 0; k < p.Horizon; k++ {
		out.Months = append(out.Months, m.AddMonths(k+1).Key())
	}

	for _, target := range p.Targets {
		ms := Milestone{Target: target, Months: -1}
		if v0 >= target {
			ms.Months = 0
			ms.Date = m.Key()
			ms.Reached = true
			ms.Already = true
			out.Milestones = append(out.Milestones, ms)
			continue
		}
		for k, v := range out.Expected {
			if v >= target {
				ms.Months = k + 1
				ms.Date = m.AddMonths(k + 1).Key()
				ms.Reached = true
				break
			}
		}
		out.Milestones = append(out.Milestones, ms)
	}

	if p.CurveValue > 0 && p.CurveYear > last.Month.Year {
		monthsRemaining := (p.CurveYear-last.Month.Year)*12 + (12 - int(last.Month.Month))
		r := math.Pow(p.CurveValue/v0, 1.0/float64(monthsRemaining)) - 1
		out.TargetCurve = TargetCurve{
			Value:           p.CurveValue,
			Year:            p.CurveYear,
			MonthsRemaining: monthsRemaining,
			RequiredMonthly: r,
			RequiredCAGR:    math.Pow(1+r, 12) - 1,
		}
		for k := range monthsRemaining {
			out.TargetCurve.CurveMonths = append(out.TargetCurve.CurveMonths, m.AddMonths(k+1).Key())
			out.TargetCurve.CurveValues = append(out.TargetCurve.CurveValues, v0*math.Pow(1+r, float64(k+1)))
		}
	}

	return out
}

func meanStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	if len(xs) < 2 {
		return mean, 0
	}
	ss := 0.0
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(ss / float64(len(xs)-1))
}

func pctIndex(q float64, n int) int {
	i := max(int(math.Round(q*float64(n-1))), 0)
	if i >= n {
		i = n - 1
	}
	return i
}
