package stats

import "conspectus/internal/domain"

type QualityReport struct {
	DuplicateMonths []DupCell
	Gaps            []AssetGaps
	StaleAssets     []StaleAssetInfo
	FutureRows      []FutureRowInfo
}

type DupCell struct {
	AssetID int32  `json:"asset_id"`
	Month   string `json:"month"`
	Count   int    `json:"count"`
}

type AssetGaps struct {
	AssetID    int32  `json:"asset_id"`
	FirstMonth string `json:"first_month"`
	LastMonth  string `json:"last_month"`
	Months     int    `json:"months"`
}

type StaleAssetInfo struct {
	AssetID     int32  `json:"asset_id"`
	Description string `json:"description"`
	LastMonth   string `json:"last_month"`
	MonthsSince int    `json:"months_since"`
}

type FutureRowInfo struct {
	AssetID int32  `json:"asset_id"`
	Month   string `json:"month"`
}

func Quality(rows []domain.SeriesRow, assets []domain.Asset, nowMonth domain.Month, staleThreshold int) QualityReport {
	rep := QualityReport{
		DuplicateMonths: []DupCell{},
		Gaps:            []AssetGaps{},
		StaleAssets:     []StaleAssetInfo{},
		FutureRows:      []FutureRowInfo{},
	}

	byAsset := map[int32]map[domain.Month]int{}
	for _, r := range rows {
		if byAsset[r.AssetID] == nil {
			byAsset[r.AssetID] = map[domain.Month]int{}
		}
		byAsset[r.AssetID][r.Month]++
		if r.Month.After(nowMonth) {
			rep.FutureRows = append(rep.FutureRows, FutureRowInfo{AssetID: r.AssetID, Month: r.Month.Key()})
		}
	}

	assetByID := map[int32]domain.Asset{}
	for _, a := range assets {
		assetByID[a.ID] = a
	}

	for id, months := range byAsset {
		for m, count := range months {
			if count > 1 {
				rep.DuplicateMonths = append(rep.DuplicateMonths, DupCell{AssetID: id, Month: m.Key(), Count: count})
			}
		}
		var first, last domain.Month
		firstSet := false
		for m := range months {
			if !firstSet || m.Before(first) {
				first = m
			}
			if !firstSet || m.After(last) {
				last = m
			}
			firstSet = true
		}
		if !firstSet {
			continue
		}
		span := monthDiff(first, last) + 1
		gaps := span - len(months)
		if gaps > 0 {
			rep.Gaps = append(rep.Gaps, AssetGaps{
				AssetID: id, FirstMonth: first.Key(), LastMonth: last.Key(), Months: gaps,
			})
		}
		if a := assetByID[id]; !a.Closed {
			since := monthDiff(last, nowMonth)
			if since > staleThreshold {
				rep.StaleAssets = append(rep.StaleAssets, StaleAssetInfo{
					AssetID: id, Description: a.Description, LastMonth: last.Key(), MonthsSince: since,
				})
			}
		}
	}
	return rep
}

func monthDiff(a, b domain.Month) int {
	return (b.Year-a.Year)*12 + int(b.Month) - int(a.Month)
}
