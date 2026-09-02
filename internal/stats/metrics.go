package stats

import (
	"math"
	"sort"

	"conspectus/internal/domain"
)

type Headline struct {
	Value     domain.Money
	Deposits  domain.Money
	Return    domain.Money
	ReturnPct *float64
	TWRR      float64
	TWRRAdj   float64
	CAGR      float64
	CAGRAdj   float64
	Months    int
}

func HeadlineFrom(s Series, c ChainResult) Headline {
	v := s.Visible()
	h := Headline{TWRR: c.TWRR, TWRRAdj: c.TWRRAdj, Months: len(v)}
	if n := len(c.Cum); n > 0 {
		h.CAGR = CAGR(c.Cum[n-1], len(c.Months))
		h.CAGRAdj = CAGR(c.CumAdj[n-1], len(c.Months))
	}
	if len(v) > 0 {
		last := v[len(v)-1]
		h.Value = last.V
		h.Deposits = last.D
		h.Return = last.V - last.D
		if last.D > 0 {
			pct := h.Return.Float() / last.D.Float()
			h.ReturnPct = &pct
		}
	}
	return h
}

type MonthReturn struct {
	Month  string
	Return float64
}

type Drawdown struct {
	Depth  float64
	Peak   string
	Trough string
}

type Risk struct {
	MonthlyMean    float64
	MonthlyMedian  float64
	VolMonthly     *float64
	VolAnnual      *float64
	Best           *MonthReturn
	Worst          *MonthReturn
	PositiveShare  float64
	MaxDDValue     *Drawdown
	MaxDDGrowth    *Drawdown
	CurrentDD      *float64
	ReturnPerRisk  *float64
	UpStreak       int
	DownStreak     int
}

func RiskFrom(s Series, c ChainResult, h Headline) Risk {
	r := Risk{}
	rets := c.monthlyReturns()
	if len(rets) == 0 {
		return r
	}

	r.MonthlyMean, _ = meanStd(rets)

	sorted := append([]float64{}, rets...)
	sort.Float64s(sorted)
	r.MonthlyMedian = median(sorted)

	if n := len(rets); n >= 2 {
		_, vol := meanStd(rets)
		r.VolMonthly = &vol
		ann := vol * math.Sqrt(12)
		r.VolAnnual = &ann
		if ann > 0 {
			rpr := h.CAGR / ann
			r.ReturnPerRisk = &rpr
		}
	}

	positive := 0
	for _, x := range rets {
		if x > 0 {
			positive++
		}
	}
	r.PositiveShare = float64(positive) / float64(len(rets))

	best, worst := extremeReturns(c)
	r.Best, r.Worst = best, worst

	upMax, downMax, up, down := 0, 0, 0, 0
	for _, x := range rets {
		switch {
		case x > 0:
			up++
			down = 0
		case x < 0:
			down++
			up = 0
		default:
			up, down = 0, 0
		}
		if up > upMax {
			upMax = up
		}
		if down > downMax {
			downMax = down
		}
	}
	r.UpStreak, r.DownStreak = upMax, downMax

	if len(c.Cum) > 0 {
		peak := c.Cum[0]
		peakIdx := 0
		maxDD := 0.0
		ddPeak, ddTrough := 0, 0
		for i, g := range c.Cum {
			if g > peak {
				peak = g
				peakIdx = i
			}
			if peak > 0 {
				dd := (peak - g) / peak
				if dd > maxDD {
					maxDD = dd
					ddPeak, ddTrough = peakIdx, i
				}
			}
		}
		if maxDD > 0 {
			r.MaxDDGrowth = &Drawdown{Depth: maxDD, Peak: c.Months[ddPeak], Trough: c.Months[ddTrough]}
		}
		if peak := c.Cum[len(c.Cum)-1]; peak > 0 {
			runPeak := 0.0
			for _, g := range c.Cum {
				if g > runPeak {
					runPeak = g
				}
			}
			cur := (runPeak - peak) / runPeak
			r.CurrentDD = &cur
		}
	}

	pts := s.ChainPoints()
	if len(pts) > 1 {
		peakV := pts[0].V
		peakM := pts[0].Month
		maxDD := 0.0
		ddPeak, ddTrough := pts[0].Month, pts[0].Month
		for _, p := range pts {
			if p.V > peakV {
				peakV = p.V
				peakM = p.Month
			}
			if peakV > 0 {
				dd := (peakV - p.V).Float() / peakV.Float()
				if dd > maxDD {
					maxDD = dd
					ddPeak, ddTrough = peakM, p.Month
				}
			}
		}
		if maxDD > 0 {
			r.MaxDDValue = &Drawdown{Depth: maxDD, Peak: ddPeak.Key(), Trough: ddTrough.Key()}
		}
	}

	return r
}

func extremeReturns(c ChainResult) (best, worst *MonthReturn) {
	start := 0
	if c.FirstLegIsSeed {
		start = 1
	}
	if len(c.G) <= start {
		return nil, nil
	}
	b, w := start, start
	for i := start; i < len(c.G); i++ {
		if c.G[i] > c.G[b] {
			b = i
		}
		if c.G[i] < c.G[w] {
			w = i
		}
	}
	best = &MonthReturn{Month: c.Months[b], Return: c.G[b] - 1}
	worst = &MonthReturn{Month: c.Months[w], Return: c.G[w] - 1}
	return best, worst
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

type Decomposition struct {
	Months      []string
	NewMoney    []float64
	Growth      []float64
	CumNewMoney []float64
	CumGrowth   []float64
}

func Decompose(s Series) Decomposition {
	d := Decomposition{
		Months: []string{}, NewMoney: []float64{}, Growth: []float64{},
		CumNewMoney: []float64{}, CumGrowth: []float64{},
	}
	visible := s.Visible()
	var prevV domain.Money
	if s.HasBaseline && s.VisibleFrom > 0 {
		prevV = s.Points[s.VisibleFrom-1].V
	} else if len(visible) > 0 {
		// The opening month has no prior month to differ from: its new money
		// would be the whole initial deposit and its growth the opening
		// valuation gap, so the split starts at the second month.
		prevV = visible[0].V
		visible = visible[1:]
	}
	if len(visible) == 0 {
		return d
	}
	cumNew, cumGrow := 0.0, 0.0
	for _, cur := range visible {
		newMoney := cur.F.Float()
		growth := (cur.V - prevV - cur.F).Float()
		d.Months = append(d.Months, cur.Month.Key())
		d.NewMoney = append(d.NewMoney, newMoney)
		d.Growth = append(d.Growth, growth)
		cumNew += newMoney
		cumGrow += growth
		d.CumNewMoney = append(d.CumNewMoney, cumNew)
		d.CumGrowth = append(d.CumGrowth, cumGrow)
		prevV = cur.V
	}
	return d
}
