package stats

import (
	"slices"
	"sort"

	"conspectus/internal/domain"
)

type Scope struct {
	AssetID int32
	ClassID int32
}

const (
	ClosedActive = "active"
	ClosedMerged = "merged"
	ClosedAll    = "all"
)

func NormalizeClosedMode(s string) string {
	switch s {
	case ClosedActive, ClosedAll:
		return s
	default:
		return ClosedMerged
	}
}

type Input struct {
	Rows       []domain.SeriesRow
	Assets     []domain.Asset
	Classes    []domain.Class
	Scope      Scope
	From       *domain.Month
	To         *domain.Month
	ClosedMode string
}

type MonthPoint struct {
	Month domain.Month
	V     domain.Money
	D     domain.Money
	F     domain.Money
	P     domain.Money
}

type Series struct {
	Points      []MonthPoint
	VisibleFrom int
	HasBaseline bool
	AssetCount  int
	ClosedCount int
}

func (s Series) Visible() []MonthPoint {
	if s.VisibleFrom >= len(s.Points) {
		return nil
	}
	return s.Points[s.VisibleFrom:]
}

func (s Series) ChainPoints() []MonthPoint {
	if s.HasBaseline && s.VisibleFrom > 0 {
		return s.Points[s.VisibleFrom-1:]
	}
	return s.Points[s.VisibleFrom:]
}

func BuildSeries(in Input) Series {
	mode := NormalizeClosedMode(in.ClosedMode)

	inScope := map[int32]bool{}
	classOf := map[int32]int32{}
	for _, a := range in.Assets {
		classOf[a.ID] = a.ClassID
		if in.Scope.AssetID != 0 && a.ID != in.Scope.AssetID {
			continue
		}
		if in.Scope.ClassID != 0 && a.ClassID != in.Scope.ClassID {
			continue
		}
		if mode == ClosedActive && a.Closed {
			continue
		}
		inScope[a.ID] = true
	}

	type snap struct {
		month    domain.Month
		deposit  domain.Money
		value    domain.Money
		payments domain.Money
	}
	byAsset := map[int32][]snap{}
	for _, r := range in.Rows {
		if !inScope[r.AssetID] {
			continue
		}
		if in.To != nil && r.Month.After(*in.To) {
			continue
		}
		byAsset[r.AssetID] = append(byAsset[r.AssetID], snap{
			month: r.Month, deposit: r.Deposit, value: r.Value, payments: r.Payments,
		})
	}
	for id := range byAsset {
		list := byAsset[id]
		sort.Slice(list, func(i, j int) bool { return list[i].month.Before(list[j].month) })
		byAsset[id] = list
	}

	if len(byAsset) == 0 {
		return Series{}
	}

	var axisStart, axisEnd domain.Month
	first := true
	closed := map[int32]bool{}
	for _, a := range in.Assets {
		if !inScope[a.ID] {
			continue
		}
		closed[a.ID] = a.Closed
	}
	for id, snaps := range byAsset {
		startM := snaps[0].month
		endM := snaps[len(snaps)-1].month
		if !closed[id] && in.To != nil && in.To.After(endM) {
			endM = *in.To
		}
		if first || startM.Before(axisStart) {
			axisStart = startM
		}
		if first || endM.After(axisEnd) {
			axisEnd = endM
		}
		first = false
	}

	agg := map[domain.Month]*MonthPoint{}
	assetIDs := make([]int32, 0, len(byAsset))
	for id := range byAsset {
		assetIDs = append(assetIDs, id)
	}
	slices.Sort(assetIDs)

	monthCell := func(m domain.Month) *MonthPoint {
		p := agg[m]
		if p == nil {
			p = &MonthPoint{Month: m}
			agg[m] = p
		}
		return p
	}

	for _, id := range assetIDs {
		snaps := byAsset[id]
		end := snaps[len(snaps)-1].month
		if !closed[id] && in.To != nil && in.To.After(end) {
			end = *in.To
		}
		idx := 0
		for m := snaps[0].month; !m.After(end); m = m.AddMonths(1) {
			for idx+1 < len(snaps) && !snaps[idx+1].month.After(m) {
				idx++
			}
			cell := monthCell(m)
			if snaps[idx].month.Equal(m) {
				cell.V += snaps[idx].value
				cell.D += snaps[idx].deposit
				cell.P += snaps[idx].payments
			} else {
				cell.V += snaps[idx].value
				cell.D += snaps[idx].deposit
			}
		}
	}

	months := make([]domain.Month, 0, len(agg))
	for m := range agg {
		months = append(months, m)
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Before(months[j]) })
	points := make([]MonthPoint, len(months))
	for i, m := range months {
		points[i] = *agg[m]
	}

	for i := len(points) - 1; i >= 0; i-- {
		if i == 0 {
			points[0].F = points[0].D
		} else {
			points[i].F = points[i].D - points[i-1].D
		}
	}

	visibleFrom := 0
	if in.From != nil {
		for i, p := range points {
			if !p.Month.Before(*in.From) {
				visibleFrom = i
				break
			}
			visibleFrom = i + 1
		}
	}

	closedCount := 0
	for id := range byAsset {
		if closed[id] {
			closedCount++
		}
	}

	return Series{
		Points:      points,
		VisibleFrom: visibleFrom,
		HasBaseline: visibleFrom > 0,
		AssetCount:  len(byAsset),
		ClosedCount: closedCount,
	}
}
