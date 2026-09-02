package stats

import (
	"math"
	"slices"
)

type ChainResult struct {
	Months         []string
	G              []float64
	GAdj           []float64
	Cum            []float64
	CumAdj         []float64
	Omitted        []int
	Notes          []string
	FirstLegIsSeed bool

	TWRR    float64
	TWRRAdj float64
}

func Chain(points []MonthPoint, hasBaseline bool) ChainResult {
	res := ChainResult{
		Months: []string{}, G: []float64{}, GAdj: []float64{},
		Cum: []float64{}, CumAdj: []float64{},
	}
	if len(points) == 0 {
		return res
	}

	cum := 1.0
	cumAdj := 1.0
	cumP := 0.0

	if !hasBaseline {
		p0 := points[0]
		if p0.D > 0 {
			g := p0.V.Float() / p0.D.Float()
			cumP += p0.P.Float()
			gAdj := (p0.V.Float() + cumP) / p0.D.Float()
			res.FirstLegIsSeed = true
			res.Months = append(res.Months, p0.Month.Key())
			res.G = append(res.G, g)
			res.GAdj = append(res.GAdj, gAdj)
			cum *= g
			cumAdj *= gAdj
			res.Cum = append(res.Cum, cum)
			res.CumAdj = append(res.CumAdj, cumAdj)
		} else {
			res.Notes = append(res.Notes,
				"first month has zero cumulative deposits (D₁=0); seed leg omitted, chain starts at the next month")
			cumP += p0.P.Float()
		}
	} else {
		cumP += points[0].P.Float()
	}

	for i := 1; i < len(points); i++ {
		prev := points[i-1]
		cur := points[i]
		idx := len(res.Months)

		g := 1.0
		denom := prev.V.Float() + cur.F.Float()
		if denom <= 0 {
			res.Omitted = append(res.Omitted, idx)
			res.Notes = append(res.Notes,
				"month "+cur.Month.Key()+": V_{m−1}+F_m ≤ 0 (portfolio emptied then refilled?); factor pinned to 1")
		} else {
			g = cur.V.Float() / denom
		}

		cumP += cur.P.Float()
		vAdj := cur.V.Float() + cumP
		vAdjPrev := prev.V.Float() + (cumP - cur.P.Float())
		gAdj := 1.0
		denomAdj := vAdjPrev + cur.F.Float()
		if denomAdj <= 0 || vAdj < 0 {
			if !slices.Contains(res.Omitted, idx) {
				res.Omitted = append(res.Omitted, idx)
			}
			res.Notes = append(res.Notes,
				"month "+cur.Month.Key()+": payments-adjusted denominator ≤ 0 or negative V′; adjusted factor pinned to 1")
		} else {
			gAdj = vAdj / denomAdj
		}

		cum *= g
		cumAdj *= gAdj
		res.Months = append(res.Months, cur.Month.Key())
		res.G = append(res.G, g)
		res.GAdj = append(res.GAdj, gAdj)
		res.Cum = append(res.Cum, cum)
		res.CumAdj = append(res.CumAdj, cumAdj)
	}

	if n := len(res.Cum); n > 0 {
		res.TWRR = res.Cum[n-1] - 1
		res.TWRRAdj = res.CumAdj[n-1] - 1
	}
	return res
}

func CAGR(cumProduct float64, nMonths int) float64 {
	if nMonths <= 0 {
		return 0
	}
	if cumProduct <= 0 {
		return -1
	}
	return math.Pow(cumProduct, 12.0/float64(nMonths)) - 1
}

func (c ChainResult) monthlyReturns() []float64 {
	start := 0
	if c.FirstLegIsSeed {
		start = 1
	}
	out := make([]float64, 0, len(c.G))
	for i := start; i < len(c.G); i++ {
		out = append(out, c.G[i]-1)
	}
	return out
}

func (c ChainResult) monthlyReturnsAdj() []float64 {
	start := 0
	if c.FirstLegIsSeed {
		start = 1
	}
	out := make([]float64, 0, len(c.GAdj))
	for i := start; i < len(c.GAdj); i++ {
		out = append(out, c.GAdj[i]-1)
	}
	return out
}
