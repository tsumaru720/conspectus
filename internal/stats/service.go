package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"conspectus/internal/domain"
)

// StatsRequest is the shared scope/range selector for the stats endpoints.
type StatsRequest struct {
	AssetID     int32
	ClassID     int32
	From        string
	To          string
	Granularity string
	Closed      string

	Horizon int
	Window  int
}

type Service struct {
	Repos    func() (domain.Repos, error)
	Settings SettingsReader
	Log      *slog.Logger
	Now      func() time.Time
}

type SettingsReader interface {
	Get(key string) string
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) repos() (domain.Repos, error) {
	if s.Repos == nil {
		return nil, fmt.Errorf("stats: storage not wired")
	}
	return s.Repos()
}

func (s *Service) loadInput(ctx context.Context, req StatsRequest) (Input, domain.Repos, error) {
	repos, err := s.repos()
	if err != nil {
		return Input{}, nil, err
	}
	q := domain.SeriesQuery{ClassID: req.ClassID}
	if req.AssetID != 0 {
		q.AssetIDs = []int32{req.AssetID}
	}
	if req.To != "" {
		to, err := domain.ParseMonth(req.To)
		if err != nil {
			return Input{}, nil, fmt.Errorf("%w: to: %v", domain.ErrValidation, err)
		}
		q.To = &to
	}
	rows, err := repos.Logs().SeriesRows(ctx, q)
	if err != nil {
		return Input{}, nil, err
	}
	assets, err := listAllAssets(ctx, repos)
	if err != nil {
		return Input{}, nil, err
	}
	classes, err := repos.Classes().List(ctx)
	if err != nil {
		return Input{}, nil, err
	}
	in := Input{
		Rows: rows, Assets: assets, Classes: classes,
		Scope:      Scope{AssetID: req.AssetID, ClassID: req.ClassID},
		ClosedMode: NormalizeClosedMode(req.Closed),
	}
	if req.From != "" {
		from, err := domain.ParseMonth(req.From)
		if err != nil {
			return Input{}, nil, fmt.Errorf("%w: from: %v", domain.ErrValidation, err)
		}
		in.From = &from
	}
	if req.To == "" {
		in.To = new(domain.MonthOf(s.now(), time.UTC))
	}
	return in, repos, nil
}

//go:fix inline
func ptrMonth(m domain.Month) *domain.Month { return new(m) }

func listAllAssets(ctx context.Context, repos domain.Repos) ([]domain.Asset, error) {
	var out []domain.Asset
	page := 1
	for {
		chunk, total, err := repos.Assets().List(ctx, domain.AssetFilter{Page: page, PerPage: 500})
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		if len(out) >= total || len(chunk) == 0 {
			break
		}
		page++
	}
	return out, nil
}

type seriesBundle struct {
	Series Series
	Chain  ChainResult
	Head   Headline
	Risk   Risk
}

func compute(s Series) seriesBundle {
	c := Chain(s.ChainPoints(), s.HasBaseline)
	h := HeadlineFrom(s, c)
	r := RiskFrom(s, c, h)
	return seriesBundle{Series: s, Chain: c, Head: h, Risk: r}
}

func (s *Service) bundle(ctx context.Context, req StatsRequest) (seriesBundle, Input, error) {
	in, _, err := s.loadInput(ctx, req)
	if err != nil {
		return seriesBundle{}, in, err
	}
	ser := BuildSeries(in)
	return compute(ser), in, nil
}

func (s *Service) Overview(ctx context.Context, req StatsRequest) (map[string]any, error) {
	b, _, err := s.bundle(ctx, req)
	if err != nil {
		return nil, err
	}
	visible := b.Series.Visible()
	out := map[string]any{
		"months":     len(visible),
		"assets":     b.Series.AssetCount,
		"closed":     b.Series.ClosedCount,
		"notes":      b.Chain.Notes,
		"sparkline":  sparkline(b.Series),
		"value":      b.Head.Value.Float(),
		"deposits":   b.Head.Deposits.Float(),
		"return":     b.Head.Return.Float(),
		"return_pct": nilZero(b.Head.ReturnPct),
		"twrr":       b.Head.TWRR,
		"twrr_adj":   b.Head.TWRRAdj,
		"cagr":       b.Head.CAGR,
	}
	delta := func(k int) map[string]any {
		n := len(visible)
		if n == 0 || k <= 0 {
			return nil
		}
		if k >= n {
			k = n - 1
		}
		prev := visible[n-1-k]
		cur := visible[n-1]
		d := map[string]any{
			"value":    (cur.V - prev.V).Float(),
			"deposits": (cur.D - prev.D).Float(),
			"return":   ((cur.V - cur.D) - (prev.V - prev.D)).Float(),
		}
		if prev.D > 0 {
			d["return_pct"] = ((cur.V.Float() - cur.D.Float()) / cur.D.Float()) - ((prev.V.Float() - prev.D.Float()) / prev.D.Float())
		}
		legs := len(b.Chain.Months)
		if k < legs {
			d["twrr"] = b.Chain.Cum[legs-1] - b.Chain.Cum[legs-1-k]
			d["twrr_adj"] = b.Chain.CumAdj[legs-1] - b.Chain.CumAdj[legs-1-k]
		}
		return d
	}
	out["delta_1m"] = delta(1)
	out["delta_1y"] = delta(12)

	return out, nil
}

func sparkline(s Series) map[string]any {
	v := s.Visible()
	months := make([]string, len(v))
	values := make([]float64, len(v))
	deposits := make([]float64, len(v))
	for i, p := range v {
		months[i] = p.Month.Key()
		values[i] = p.V.Float()
		deposits[i] = p.D.Float()
	}
	return map[string]any{"months": months, "value": values, "deposits": deposits}
}

func (s *Service) Series(ctx context.Context, req StatsRequest, metric string) (map[string]any, error) {
	b, _, err := s.bundle(ctx, req)
	if err != nil {
		return nil, err
	}
	g := granularityKeyer(req.Granularity)
	out := map[string]any{}

	switch metric {
	case "value", "deposits":
		months, vals := groupLast(g, b.Series, func(p MonthPoint) float64 {
			if metric == "value" {
				return p.V.Float()
			}
			return p.D.Float()
		})
		out["months"] = months
		out["data"] = vals
	case "return":
		months, vals := groupLast(g, b.Series, func(p MonthPoint) float64 { return (p.V - p.D).Float() })
		out["months"], out["data"] = months, vals
	case "return_pct":
		months, vals := groupLastOmit(g, b.Series, func(p MonthPoint) *float64 {
			if p.D <= 0 {
				return nil
			}
			v := (p.V.Float() - p.D.Float()) / p.D.Float()
			return &v
		})
		out["months"], out["data"] = months, vals
	case "twrr", "twrr_adj":
		months, cum := regroupChain(g, b.Chain, metric == "twrr_adj")
		out["months"] = months
		out["data"] = cum
	case "growth_index":
		months, cum := regroupChain(g, b.Chain, true)
		out["months"] = months
		out["data"] = cum
	case "decomposition":
		d := Decompose(b.Series)
		out["months"] = d.Months
		out["new_money"] = d.NewMoney
		out["growth"] = d.Growth
		out["cum_new_money"] = d.CumNewMoney
		out["cum_growth"] = d.CumGrowth
	case "payments":
		months, vals := groupSum(g, b.Series, func(p MonthPoint) float64 { return p.P.Float() })
		out["months"], out["data"] = months, vals
	default:
		return nil, fmt.Errorf("%w: unknown metric %q (value|deposits|return|return_pct|twrr|twrr_adj|growth_index|decomposition|payments)",
			domain.ErrValidation, metric)
	}
	out["notes"] = b.Chain.Notes
	return out, nil
}

func (s *Service) Breakdown(ctx context.Context, req StatsRequest) (map[string]any, error) {
	in, _, err := s.loadInput(ctx, req)
	if err != nil {
		return nil, err
	}
	classes, assets, conc, closed := Breakdown(in)

	classRows := make([]any, 0, len(classes))
	for _, c := range classes {
		classRows = append(classRows, map[string]any{
			"class_id": c.ClassID, "description": c.Description,
			"value": c.Value.Float(), "deposits": c.Deposits.Float(),
			"return": c.Return.Float(), "return_pct": nilZero(c.ReturnPct),
			"twrr_adj": c.TWRRAdj, "share": c.Share,
			"legs": c.Legs, "short_history": c.ShortHistory,
		})
	}
	assetRows := make([]any, 0, len(assets))
	for _, a := range assets {
		assetRows = append(assetRows, map[string]any{
			"asset_id": a.AssetID, "class_id": a.ClassID, "description": a.Description,
			"value": a.Value.Float(), "deposits": a.Deposits.Float(),
			"return": a.Return.Float(), "return_pct": nilZero(a.ReturnPct),
			"twrr_adj": a.TWRRAdj, "share": a.Share,
			"legs": a.Legs, "short_history": a.ShortHistory,
		})
	}
	closedAssets := make([]any, 0, len(closed.Assets))
	for _, a := range closed.Assets {
		closedAssets = append(closedAssets, map[string]any{
			"asset_id": a.AssetID, "description": a.Description, "class_id": a.ClassID,
			"deposits": a.Deposits.Float(), "value": a.Value.Float(),
			"gain": a.Gain.Float(), "last_month": a.LastMonth.Key(),
		})
	}
	months, overTime := AllocationOverTime(in)
	seriesAny := make(map[string]any, len(overTime))
	for name, vals := range overTime {
		seriesAny[name] = vals
	}

	assetMonths, assetOverTime := AllocationOverTimeAssets(in)
	assetSeriesAny := make(map[string]any, len(assetOverTime))
	for name, vals := range assetOverTime {
		assetSeriesAny[name] = vals
	}

	out := map[string]any{
		"classes": classRows,
		"assets":  assetRows,
		"concentration": map[string]any{
			"largest_share": conc.LargestShare, "largest_asset": conc.LargestAsset,
			"hhi": conc.HHI, "effective_n": conc.EffectiveN,
		},
		"closed": map[string]any{
			"count":          closed.Count,
			"total_deposits": closed.TotalDeposits.Float(),
			"total_value":    closed.TotalValue.Float(),
			"realized_gain":  closed.RealizedGain.Float(),
			"assets":         closedAssets,
		},
		"allocation_over_time": map[string]any{
			"months": months,
			"series": seriesAny,
		},
		"assets_over_time": map[string]any{
			"months": assetMonths,
			"series": assetSeriesAny,
		},
	}
	return out, nil
}

func (s *Service) Analytics(ctx context.Context, req StatsRequest) (map[string]any, error) {
	b, _, err := s.bundle(ctx, req)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"yearly":        yearlyTable(b),
		"records":       recordsMap(b.Risk),
		"histogram":     histogram(b.Chain),
		"heatmap":       heatmap(b.Chain),
		"decomposition": decomposeMap(b.Series),
	}
	return out, nil
}

func yearlyTable(b seriesBundle) []map[string]any {
	type yearAcc struct {
		months                     []int
		opening, lastV, sumF, sumP domain.Money
		prod, prodAdj              float64
		legs                       int
		hasPrior                   bool
	}
	years := map[int]*yearAcc{}
	visible := b.Series.Visible()
	prevV := domain.Money(0)
	hasPrior := false
	if len(visible) > 0 {
		if b.Series.HasBaseline && b.Series.VisibleFrom > 0 {
			prevV = b.Series.Points[b.Series.VisibleFrom-1].V
			hasPrior = true
		} else {
			prevV = visible[0].V - visible[0].F
		}
	}
	for i, p := range visible {
		y := years[p.Month.Year]
		if y == nil {
			y = &yearAcc{prod: 1, prodAdj: 1, opening: prevV, hasPrior: hasPrior}
			years[p.Month.Year] = y
		}
		y.lastV = p.V
		y.sumF += p.F
		y.sumP += p.P
		y.months = append(y.months, int(p.Month.Month))
		if i < len(b.Chain.G) {
			legIdx := i
			if b.Series.HasBaseline {
				legIdx = i - 1
			}
			if legIdx >= 0 && legIdx < len(b.Chain.G) {
				y.prod *= b.Chain.G[legIdx]
				y.prodAdj *= b.Chain.GAdj[legIdx]
				y.legs++
			}
		}
		prevV = p.V
		hasPrior = true
	}
	keys := make([]int, 0, len(years))
	for y := range years {
		keys = append(keys, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))
	rows := make([]map[string]any, 0, len(keys))
	for _, y := range keys {
		a := years[y]
		change := any(nil)
		if a.hasPrior && a.opening > 0 {
			c := a.lastV.Float()/a.opening.Float() - 1
			change = c
		}
		rows = append(rows, map[string]any{
			"year":       y,
			"start":      a.opening.Float(),
			"end":        a.lastV.Float(),
			"change_pct": change,
			"deposits":   a.sumF.Float(),
			"growth":     (a.lastV - a.opening - a.sumF + a.sumP).Float(),
			"income":     a.sumP.Float(),
			"twrr":       a.prod - 1,
			"twrr_adj":   a.prodAdj - 1,
		})
	}
	return rows
}

func recordsMap(r Risk) map[string]any {
	out := map[string]any{
		"up_streak":   r.UpStreak,
		"down_streak": r.DownStreak,
	}
	if r.Best != nil {
		out["best_month"] = map[string]any{"month": r.Best.Month, "return": r.Best.Return}
	}
	if r.Worst != nil {
		out["worst_month"] = map[string]any{"month": r.Worst.Month, "return": r.Worst.Return}
	}
	if r.MaxDDGrowth != nil {
		out["max_drawdown_growth"] = map[string]any{"depth": r.MaxDDGrowth.Depth, "peak": r.MaxDDGrowth.Peak, "trough": r.MaxDDGrowth.Trough}
	}
	if r.MaxDDValue != nil {
		out["max_drawdown_value"] = map[string]any{"depth": r.MaxDDValue.Depth, "peak": r.MaxDDValue.Peak, "trough": r.MaxDDValue.Trough, "caveat": "distorted by deposits - see the growth-index drawdown"}
	}
	if r.CurrentDD != nil {
		out["current_drawdown"] = *r.CurrentDD
	}
	if r.VolAnnual != nil {
		out["volatility_monthly"] = *r.VolMonthly
		out["volatility_annual"] = *r.VolAnnual
	}
	if r.ReturnPerRisk != nil {
		out["return_per_risk"] = *r.ReturnPerRisk
	}
	out["positive_share"] = r.PositiveShare
	out["monthly_mean"] = r.MonthlyMean
	out["monthly_median"] = r.MonthlyMedian
	return out
}

func histogram(c ChainResult) map[string]any {
	rets := c.monthlyReturns()
	if len(rets) == 0 {
		return map[string]any{"bins": []any{}, "counts": []int{}}
	}
	nBuckets := max(int(math.Ceil(math.Sqrt(float64(len(rets))))), 3)
	if nBuckets > 24 {
		nBuckets = 24
	}
	lo, hi := rets[0], rets[0]
	for _, x := range rets {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	if hi == lo {
		hi = lo + 1e-9
	}
	width := (hi - lo) / float64(nBuckets)
	counts := make([]int, nBuckets)
	for _, x := range rets {
		idx := int((x - lo) / width)
		if idx >= nBuckets {
			idx = nBuckets - 1
		}
		if idx < 0 {
			idx = 0
		}
		counts[idx]++
	}
	bins := make([]string, nBuckets)
	for i := range bins {
		bins[i] = fmt.Sprintf("%.2f%%..%.2f%%", (lo+float64(i)*width)*100, (lo+float64(i+1)*width)*100)
	}
	return map[string]any{"bins": bins, "counts": counts}
}

func heatmap(c ChainResult) map[string]any {
	grid := map[string]any{}
	years := map[int]bool{}
	for i, mk := range c.Months {
		var y int
		var mm int
		fmt.Sscanf(mk, "%d-%d", &y, &mm)
		years[y] = true
		key := fmt.Sprintf("%d", y)
		row, _ := grid[key].([]any)
		if row == nil {
			row = make([]any, 12)
			for j := range row {
				row[j] = nil
			}
		}
		row[mm-1] = c.G[i] - 1
		grid[key] = row
	}
	ys := make([]int, 0, len(years))
	for y := range years {
		ys = append(ys, y)
	}
	sort.Ints(ys)
	out := map[string]any{"years": ys, "cells": grid}
	return out
}

func decomposeMap(s Series) map[string]any {
	d := Decompose(s)
	return map[string]any{
		"months": d.Months, "new_money": d.NewMoney, "growth": d.Growth,
		"cum_new_money": d.CumNewMoney, "cum_growth": d.CumGrowth,
	}
}

func (s *Service) Projections(ctx context.Context, req StatsRequest) (map[string]any, error) {
	in, _, err := s.loadInput(ctx, req)
	if err != nil {
		return nil, err
	}
	params := ProjectionParams{
		DepositWin: "trailing12",
		Horizon:    req.Horizon,
		Window:     req.Window,
	}
	if s.Settings != nil {
		params.Targets = parseTargets(s.Settings.Get("projection_targets"))
		params.CurveValue, params.CurveYear = parseCurve(s.Settings.Get("projection_target_curve"))
	}
	if params.Horizon == 0 {
		params.Horizon = 120
	}
	p := Project(in, params)

	milestones := make([]any, 0, len(p.Milestones))
	for _, ms := range p.Milestones {
		milestones = append(milestones, map[string]any{
			"target": ms.Target, "months": ms.Months, "date": ms.Date,
			"reached": ms.Reached, "already": ms.Already,
		})
	}
	fromMonth := p.FromMonth.Key()
	if p.FromMonth.Year == 0 {
		fromMonth = ""
	}
	out := map[string]any{
		"horizon_months": p.HorizonMonths,
		"from_month":     fromMonth,
		"months":         p.Months,
		"expected":       p.Expected,
		"upper":          p.Upper,
		"lower":          p.Lower,
		"cagr_path":      p.CAGRPath,
		"milestones":     milestones,
		"assumptions":    p.Assumptions,
		"caveat":         "Extrapolations of past behaviour - not promises.",
	}
	if p.TargetCurve.Value > 0 {
		out["target_curve"] = map[string]any{
			"value": p.TargetCurve.Value, "year": p.TargetCurve.Year,
			"months_remaining": p.TargetCurve.MonthsRemaining,
			"required_monthly": p.TargetCurve.RequiredMonthly,
			"required_cagr":    p.TargetCurve.RequiredCAGR,
			"curve_months":     p.TargetCurve.CurveMonths,
			"curve_values":     p.TargetCurve.CurveValues,
		}
	}
	return out, nil
}

func parseTargets(raw string) []float64 {
	var out []float64
	if raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func parseCurve(raw string) (float64, int) {
	var v struct {
		Value float64 `json:"value"`
		Year  int     `json:"year"`
	}
	if raw == "" {
		return 0, 0
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return 0, 0
	}
	return v.Value, v.Year
}

func (s *Service) Income(ctx context.Context, req StatsRequest) (map[string]any, error) {
	in, _, err := s.loadInput(ctx, req)
	if err != nil {
		return nil, err
	}
	rep := Income(in)
	out := map[string]any{
		"total":       rep.Total.Float(),
		"zero_months": rep.ZeroMonths,
		"by_month":    monthAmounts(rep.ByMonth),
		"by_year":     yearAmounts(rep.ByYear),
	}
	byClass := make([]map[string]any, 0, len(rep.ByClass))
	for _, c := range rep.ByClass {
		byClass = append(byClass, map[string]any{"class_id": c.ClassID, "description": c.Description, "amount": c.Amount.Float()})
	}
	out["by_class"] = byClass
	top := make([]map[string]any, 0, len(rep.TopAssets))
	for _, a := range rep.TopAssets {
		top = append(top, map[string]any{"asset_id": a.AssetID, "description": a.Description, "amount": a.Amount.Float()})
	}
	out["top_assets"] = top
	if rep.TTMYield != nil {
		out["ttm_yield"] = *rep.TTMYield
	}
	if rep.YoYGrowth != nil {
		out["yoy_growth"] = *rep.YoYGrowth
	}
	idle := make([]map[string]any, 0, len(rep.IdleCash))
	for _, a := range rep.IdleCash {
		idle = append(idle, map[string]any{"asset_id": a.AssetID, "description": a.Description, "value": a.Value.Float(), "twrr_12m": a.TWRR12m})
	}
	out["idle_cash"] = idle
	return out, nil
}

func monthAmounts(list []MonthAmount) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]any{"month": m.Month, "amount": m.Amount.Float()})
	}
	return out
}

func yearAmounts(list []YearAmount) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, y := range list {
		out = append(out, map[string]any{"year": y.Year, "amount": y.Amount.Float()})
	}
	return out
}

func (s *Service) Quality(ctx context.Context) (map[string]any, error) {
	repos, err := s.repos()
	if err != nil {
		return nil, err
	}
	rows, err := repos.Logs().SeriesRows(ctx, domain.SeriesQuery{})
	if err != nil {
		return nil, err
	}
	assets, err := listAllAssets(ctx, repos)
	if err != nil {
		return nil, err
	}
	nowMonth := domain.MonthOf(s.now(), time.UTC)
	rep := Quality(rows, assets, nowMonth, 13)

	dups := make([]map[string]any, 0, len(rep.DuplicateMonths))
	for _, d := range rep.DuplicateMonths {
		dups = append(dups, map[string]any{"asset_id": d.AssetID, "month": d.Month, "count": d.Count})
	}
	gaps := make([]map[string]any, 0, len(rep.Gaps))
	for _, g := range rep.Gaps {
		gaps = append(gaps, map[string]any{"asset_id": g.AssetID, "first_month": g.FirstMonth, "last_month": g.LastMonth, "months": g.Months})
	}
	stale := make([]map[string]any, 0, len(rep.StaleAssets))
	for _, a := range rep.StaleAssets {
		stale = append(stale, map[string]any{"asset_id": a.AssetID, "description": a.Description, "last_month": a.LastMonth, "months_since": a.MonthsSince})
	}
	future := make([]map[string]any, 0, len(rep.FutureRows))
	for _, f := range rep.FutureRows {
		future = append(future, map[string]any{"asset_id": f.AssetID, "month": f.Month})
	}
	out := map[string]any{
		"duplicate_months": dups,
		"gaps":             gaps,
		"stale_assets":     stale,
		"future_rows":      future,
	}
	return out, nil
}

func nilZero(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func granularityKeyer(g string) func(domain.Month) string {
	switch strings.ToLower(g) {
	case "quarter":
		return func(m domain.Month) string { return fmt.Sprintf("%d-Q%d", m.Year, m.Quarter()) }
	case "year":
		return func(m domain.Month) string { return fmt.Sprintf("%d", m.Year) }
	default:
		return func(m domain.Month) string { return m.Key() }
	}
}

func groupLast(g func(domain.Month) string, s Series, val func(MonthPoint) float64) ([]string, []float64) {
	months, vals := []string{}, []float64{}
	for _, p := range s.Visible() {
		k := g(p.Month)
		if len(months) > 0 && months[len(months)-1] == k {
			vals[len(vals)-1] = val(p)
		} else {
			months = append(months, k)
			vals = append(vals, val(p))
		}
	}
	return months, vals
}

func groupLastOmit(g func(domain.Month) string, s Series, val func(MonthPoint) *float64) ([]string, []any) {
	months, vals := []string{}, []any{}
	for _, p := range s.Visible() {
		k := g(p.Month)
		v := val(p)
		var fv any
		if v != nil {
			fv = *v
		}
		if len(months) > 0 && months[len(months)-1] == k {
			vals[len(vals)-1] = fv
		} else {
			months = append(months, k)
			vals = append(vals, fv)
		}
	}
	return months, vals
}

func groupSum(g func(domain.Month) string, s Series, val func(MonthPoint) float64) ([]string, []float64) {
	months, vals := []string{}, []float64{}
	for _, p := range s.Visible() {
		k := g(p.Month)
		if len(months) > 0 && months[len(months)-1] == k {
			vals[len(vals)-1] += val(p)
		} else {
			months = append(months, k)
			vals = append(vals, val(p))
		}
	}
	return months, vals
}

func regroupChain(g func(domain.Month) string, c ChainResult, adj bool) ([]string, []float64) {
	src := c.Cum
	if adj {
		src = c.CumAdj
	}
	months, vals := []string{}, []float64{}
	for i, mk := range c.Months {
		var m domain.Month
		if mm, err := domain.ParseMonth(mk); err == nil {
			m = mm
		}
		k := g(m)
		if len(months) > 0 && months[len(months)-1] == k {
			vals[len(vals)-1] = src[i]
		} else {
			months = append(months, k)
			vals = append(vals, src[i])
		}
	}
	return months, vals
}
