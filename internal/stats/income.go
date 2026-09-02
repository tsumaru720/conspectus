package stats

import (
	"sort"

	"conspectus/internal/domain"
)

type IncomeReport struct {
	Total      domain.Money
	ByMonth    []MonthAmount
	ByYear     []YearAmount
	ByClass    []ClassAmount
	TopAssets  []AssetAmount
	TTMYield   *float64
	YoYGrowth  *float64
	ZeroMonths int
	IdleCash   []IdleAsset
}

type MonthAmount struct {
	Month  string
	Amount domain.Money
}

type YearAmount struct {
	Year   int
	Amount domain.Money
}

type ClassAmount struct {
	ClassID     int32
	Description string
	Amount      domain.Money
}

type AssetAmount struct {
	AssetID     int32
	Description string
	Amount      domain.Money
}

type IdleAsset struct {
	AssetID     int32
	Description string
	Value       domain.Money
	TWRR12m     float64
}

func Income(in Input) IncomeReport {
	rep := IncomeReport{}
	s := BuildSeries(in)
	visible := s.Visible()

	byYear := map[int]domain.Money{}
	for _, p := range visible {
		rep.Total += p.P
		rep.ByMonth = append(rep.ByMonth, MonthAmount{Month: p.Month.Key(), Amount: p.P})
		byYear[p.Month.Year] += p.P
		if p.P == 0 {
			rep.ZeroMonths++
		}
	}
	years := make([]int, 0, len(byYear))
	for y := range byYear {
		years = append(years, y)
	}
	sort.Ints(years)
	for _, y := range years {
		rep.ByYear = append(rep.ByYear, YearAmount{Year: y, Amount: byYear[y]})
	}

	for _, c := range in.Classes {
		cs := BuildSeries(Input{Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{ClassID: c.ID}, From: in.From, To: in.To, ClosedMode: NormalizeClosedMode(in.ClosedMode)})
		var total domain.Money
		for _, p := range cs.Visible() {
			total += p.P
		}
		if total != 0 {
			rep.ByClass = append(rep.ByClass, ClassAmount{ClassID: c.ID, Description: c.Description, Amount: total})
		}
	}
	for _, a := range in.Assets {
		as := BuildSeries(Input{Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{AssetID: a.ID}, From: in.From, To: in.To, ClosedMode: NormalizeClosedMode(in.ClosedMode)})
		var total domain.Money
		for _, p := range as.Visible() {
			total += p.P
		}
		if total != 0 {
			rep.TopAssets = append(rep.TopAssets, AssetAmount{AssetID: a.ID, Description: a.Description, Amount: total})
		}
	}
	sortAssetAmounts(rep.TopAssets)

	if n := len(visible); n > 0 {
		start := max(n-12, 0)
		window := visible[start:]
		var sumP, sumV domain.Money
		for _, p := range window {
			sumP += p.P
			sumV += p.V
		}
		avgV := sumV / domain.Money(len(window))
		if avgV > 0 {
			y := sumP.Float() / avgV.Float()
			rep.TTMYield = &y
		}
		if n >= 13 {
			this12, prev12 := domain.Money(0), domain.Money(0)
			for i := n - 12; i < n; i++ {
				this12 += visible[i].P
			}
			for i := n - 24; i < n-12; i++ {
				if i >= 0 {
					prev12 += visible[i].P
				}
			}
			if prev12 > 0 {
				g := (this12.Float() - prev12.Float()) / prev12.Float()
				rep.YoYGrowth = &g
			}
		}
	}

	for _, a := range in.Assets {
		if a.Closed {
			continue
		}
		to := in.To
		if to == nil && len(visible) > 0 {
			t := visible[len(visible)-1].Month
			to = &t
		}
		if to == nil {
			continue
		}
		from := to.AddMonths(-11)
		as := BuildSeries(Input{Rows: in.Rows, Assets: in.Assets, Classes: in.Classes,
			Scope: Scope{AssetID: a.ID}, From: &from, To: to, ClosedMode: ClosedAll})
		v := as.Visible()
		if len(v) == 0 || v[len(v)-1].V <= 0 {
			continue
		}
		c := Chain(as.ChainPoints(), as.HasBaseline)
		if c.TWRR > -0.001 && c.TWRR < 0.001 {
			rep.IdleCash = append(rep.IdleCash, IdleAsset{
				AssetID: a.ID, Description: a.Description,
				Value: v[len(v)-1].V, TWRR12m: c.TWRR,
			})
		}
	}

	return rep
}

func sortAssetAmounts(list []AssetAmount) {
	sort.Slice(list, func(i, j int) bool { return list[i].Amount > list[j].Amount })
}
