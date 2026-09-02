package stats

import (
	"sort"
	"strings"

	"conspectus/internal/domain"
)

type ClassStat struct {
	ClassID      int32
	Description  string
	Value        domain.Money
	Deposits     domain.Money
	Return       domain.Money
	ReturnPct    *float64
	TWRRAdj      float64
	Share        float64
	Legs         int
	ShortHistory bool
}

type AssetStat struct {
	AssetID      int32
	ClassID      int32
	Description  string
	Closed       bool
	Value        domain.Money
	Deposits     domain.Money
	Return       domain.Money
	ReturnPct    *float64
	TWRRAdj      float64
	Share        float64
	Legs         int
	ShortHistory bool
}

type Concentration struct {
	LargestShare   float64
	LargestAsset   string
	LargestAssetID int32
	HHI            float64
	EffectiveN     float64
}

type ClosedSummary struct {
	Count         int
	TotalDeposits domain.Money
	TotalValue    domain.Money
	RealizedGain  domain.Money
	Assets        []ClosedAsset
}

type ClosedAsset struct {
	AssetID     int32
	Description string
	ClassID     int32
	Deposits    domain.Money
	Value       domain.Money
	Gain        domain.Money
	LastMonth   domain.Month
}

func Breakdown(in Input) (classes []ClassStat, assets []AssetStat, conc Concentration, closed ClosedSummary) {
	base := BuildSeries(in)
	totalV := domain.Money(0)
	if v := base.Visible(); len(v) > 0 {
		totalV = v[len(v)-1].V
	}

	for _, c := range in.Classes {
		if in.Scope.ClassID != 0 && c.ID != in.Scope.ClassID {
			continue
		}
		cs := BuildSeries(Input{
			Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{ClassID: c.ID}, From: in.From, To: in.To,
			ClosedMode: ClosedActive,
		})
		h := headlineOf(cs)
		if h.Months == 0 {
			continue
		}
		stat := ClassStat{
			ClassID: c.ID, Description: c.Description,
			Value: h.Value, Deposits: h.Deposits, Return: h.Return,
			ReturnPct: h.ReturnPct, TWRRAdj: h.TWRRAdj, Legs: h.Months,
			ShortHistory: h.Months < 6,
		}
		if totalV > 0 {
			stat.Share = h.Value.Float() / totalV.Float()
		}
		classes = append(classes, stat)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].Value > classes[j].Value })

	for _, a := range in.Assets {
		if in.Scope.AssetID != 0 && a.ID != in.Scope.AssetID {
			continue
		}
		if in.Scope.ClassID != 0 && a.ClassID != in.Scope.ClassID {
			continue
		}
		if a.Closed {
			continue
		}
		as := BuildSeries(Input{
			Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{AssetID: a.ID}, From: in.From, To: in.To,
			ClosedMode: ClosedActive,
		})
		h := headlineOf(as)
		if h.Months == 0 {
			continue
		}
		stat := AssetStat{
			AssetID: a.ID, ClassID: a.ClassID, Description: a.Description,
			Value: h.Value, Deposits: h.Deposits, Return: h.Return,
			ReturnPct: h.ReturnPct, TWRRAdj: h.TWRRAdj, Legs: h.Months,
			ShortHistory: h.Months < 6,
		}
		if totalV > 0 {
			stat.Share = h.Value.Float() / totalV.Float()
		}
		assets = append(assets, stat)
	}
	sort.Slice(assets, func(i, j int) bool {
		return strings.ToLower(assets[i].Description) < strings.ToLower(assets[j].Description)
	})

	hhi := 0.0
	for _, a := range assets {
		hhi += a.Share * a.Share
		if a.Share > conc.LargestShare {
			conc.LargestShare = a.Share
			conc.LargestAsset = a.Description
			conc.LargestAssetID = a.AssetID
		}
	}
	conc.HHI = hhi
	if hhi > 0 {
		conc.EffectiveN = 1 / hhi
	}

	byAsset := map[int32][]domain.SeriesRow{}
	for _, r := range in.Rows {
		byAsset[r.AssetID] = append(byAsset[r.AssetID], r)
	}
	assetByID := map[int32]domain.Asset{}
	for _, a := range in.Assets {
		assetByID[a.ID] = a
	}
	for _, a := range in.Assets {
		if !a.Closed {
			continue
		}
		if in.Scope.ClassID != 0 && a.ClassID != in.Scope.ClassID {
			continue
		}
		rows := byAsset[a.ID]
		if len(rows) == 0 {
			continue
		}
		last := rows[0]
		for _, r := range rows[1:] {
			if r.Month.After(last.Month) {
				last = r
			}
		}
		closed.TotalDeposits += last.Deposit
		closed.TotalValue += last.Value
		closed.Count++
		closed.Assets = append(closed.Assets, ClosedAsset{
			AssetID: a.ID, Description: a.Description, ClassID: a.ClassID,
			Deposits: last.Deposit, Value: last.Value,
			Gain: last.Value - last.Deposit, LastMonth: last.Month,
		})
	}
	closed.RealizedGain = closed.TotalValue - closed.TotalDeposits
	sort.Slice(closed.Assets, func(i, j int) bool { return closed.Assets[i].LastMonth.After(closed.Assets[j].LastMonth) })
	return classes, assets, conc, closed
}

func headlineOf(s Series) Headline {
	return HeadlineFrom(s, Chain(s.ChainPoints(), s.HasBaseline))
}

func AllocationOverTime(in Input) (months []string, byClass map[string][]float64) {
	byClass = map[string][]float64{}
	s := BuildSeries(Input{Rows: in.Rows, Assets: in.Assets, Classes: in.Classes, Scope: in.Scope, From: in.From, To: in.To, ClosedMode: ClosedMerged})
	months = monthKeys(s.Visible())
	if len(months) == 0 {
		return months, byClass
	}
	for _, c := range in.Classes {
		if in.Scope.ClassID != 0 && c.ID != in.Scope.ClassID {
			continue
		}
		cs := BuildSeries(Input{
			Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{ClassID: c.ID, AssetID: in.Scope.AssetID}, From: in.From, To: in.To,
			ClosedMode: ClosedMerged,
		})
		byMonth := map[string]float64{}
		for _, p := range cs.Visible() {
			byMonth[p.Month.Key()] = p.V.Float()
		}
		vals := make([]float64, len(months))
		for i, mk := range months {
			vals[i] = byMonth[mk]
		}
		if anyNonZero(vals) {
			byClass[c.Description] = vals
		}
	}
	return months, byClass
}

func monthKeys(pts []MonthPoint) []string {
	out := make([]string, len(pts))
	for i, p := range pts {
		out[i] = p.Month.Key()
	}
	return out
}

func AllocationOverTimeAssets(in Input) (months []string, byAsset map[string][]float64) {
	byAsset = map[string][]float64{}
	s := BuildSeries(Input{Rows: in.Rows, Assets: in.Assets, Classes: in.Classes, Scope: in.Scope, From: in.From, To: in.To, ClosedMode: ClosedAll})
	months = monthKeys(s.Visible())
	if len(months) == 0 {
		return months, byAsset
	}
	descr := map[int32]string{}
	for _, a := range in.Assets {
		descr[a.ID] = a.Description
	}
	for _, a := range in.Assets {
		if in.Scope.AssetID != 0 && a.ID != in.Scope.AssetID {
			continue
		}
		if in.Scope.ClassID != 0 && a.ClassID != in.Scope.ClassID {
			continue
		}
		as := BuildSeries(Input{
			Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{AssetID: a.ID}, From: in.From, To: in.To,
			ClosedMode: ClosedAll,
		})
		byMonth := map[string]float64{}
		for _, p := range as.Visible() {
			byMonth[p.Month.Key()] = p.V.Float()
		}
		vals := make([]float64, len(months))
		for i, mk := range months {
			vals[i] = byMonth[mk]
		}
		if anyNonZero(vals) {
			byAsset[descr[a.ID]] = vals
		}
	}
	return months, byAsset
}

func anyNonZero(vals []float64) bool {
	for _, v := range vals {
		if v != 0 {
			return true
		}
	}
	return false
}
