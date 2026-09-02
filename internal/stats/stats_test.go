package stats

import (
	"math"
	"testing"

	"conspectus/internal/domain"
)

func m(major string) domain.Money {
	v, err := domain.ParseMoney(major)
	if err != nil {
		panic(err)
	}
	return v
}

func month(s string) domain.Month {
	mh, err := domain.ParseMonth(s)
	if err != nil {
		panic(err)
	}
	return mh
}

func TestWorkedExample(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2026-01"), Deposit: m("2000"), Value: m("2000")},
		{AssetID: 1, Month: month("2026-02"), Deposit: m("2000"), Value: m("2004")},
		{AssetID: 1, Month: month("2026-03"), Deposit: m("2500"), Value: m("2506")},
		{AssetID: 1, Month: month("2026-04"), Deposit: m("2500"), Value: m("2500"), Payments: m("8")},
	}
	assets := []domain.Asset{{ID: 1, ClassID: 1, Description: "Saver"}}
	s := BuildSeries(Input{Rows: rows, Assets: assets})
	c := Chain(s.ChainPoints(), s.HasBaseline)

	if len(s.Points) != 4 {
		t.Fatalf("points = %d, want 4", len(s.Points))
	}
	wantG := []float64{1.0, 2004.0 / 2000.0, 2506.0 / 2504.0, 2500.0 / 2506.0}
	wantGAdj := []float64{1.0, 2004.0 / 2000.0, 2506.0 / 2504.0, 2508.0 / 2506.0}
	for i, want := range wantG {
		if got := c.G[i]; math.Abs(got-want) > 1e-9 {
			t.Errorf("G[%d] = %.6f, want %.6f", i, got, want)
		}
	}
	for i, want := range wantGAdj {
		if got := c.GAdj[i]; math.Abs(got-want) > 1e-9 {
			t.Errorf("GAdj[%d] = %.6f, want %.6f", i, got, want)
		}
	}
	wantTWRR := 1.0*(2004.0/2000.0)*(2506.0/2504.0)*(2500.0/2506.0) - 1
	wantTWRRAdj := 1.0*(2004.0/2000.0)*(2506.0/2504.0)*(2508.0/2506.0) - 1
	if math.Abs(c.TWRR-wantTWRR) > 1e-12 {
		t.Errorf("TWRR = %.8f, want %.8f", c.TWRR, wantTWRR)
	}
	if math.Abs(c.TWRRAdj-wantTWRRAdj) > 1e-12 {
		t.Errorf("TWRRAdj = %.8f, want %.8f", c.TWRRAdj, wantTWRRAdj)
	}

	h := HeadlineFrom(s, c)
	if h.Return != 0 {
		t.Errorf("Return = %v, want 0", h.Return)
	}
	if h.ReturnPct == nil || *h.ReturnPct != 0 {
		t.Errorf("ReturnPct = %v, want 0", h.ReturnPct)
	}

	d := Decompose(s)
	// 2026-01 is the seed month: with no prior month to differ from, the
	// split starts at 2026-02.
	if len(d.Months) != 3 || d.Months[0] != "2026-02" {
		t.Fatalf("decomposition months = %v, want 2026-02..2026-04", d.Months)
	}
	var sumNew, sumGrow float64
	for i := range d.Months {
		sumNew += d.NewMoney[i]
		sumGrow += d.Growth[i]
	}
	if math.Abs(sumNew-500.0) > 1e-9 {
		t.Errorf("Σ new money = %.2f, want 500", sumNew)
	}
	if math.Abs((sumNew+sumGrow)-500.0) > 1e-9 {
		t.Errorf("Σ new money + Σ growth = %.2f, want 500 (V growth since the seed)", sumNew+sumGrow)
	}
}

func TestGapCarryForward(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("1000"), Value: m("1020")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}})
	if len(s.Points) != 3 {
		t.Fatalf("points = %d, want 3 (Feb carried)", len(s.Points))
	}
	feb := s.Points[1]
	if feb.V != m("1000") || feb.D != m("1000") || feb.F != 0 {
		t.Errorf("February carried point = %+v, want V=D=1000, F=0", feb)
	}
	c := Chain(s.ChainPoints(), s.HasBaseline)
	if len(c.G) != 3 {
		t.Fatalf("legs = %d, want 3 (seed, carried 1.0, growth)", len(c.G))
	}
	if math.Abs(c.G[1]-1.0) > 1e-12 {
		t.Errorf("carried leg = %.6f, want 1.0", c.G[1])
	}
	if math.Abs(c.G[2]-1.02) > 1e-12 {
		t.Errorf("growth leg = %.6f, want 1.02", c.G[2])
	}
	if math.Abs(c.TWRR-0.02) > 1e-12 {
		t.Errorf("TWRR = %.6f, want 0.02", c.TWRR)
	}
}

func TestAssetAddedMidHistory(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1000"), Value: m("1010")},
		{AssetID: 2, Month: month("2025-02"), Deposit: m("500"), Value: m("500")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}, {ID: 2}}})
	if len(s.Points) != 2 {
		t.Fatalf("points = %d, want 2", len(s.Points))
	}
	feb := s.Points[1]
	if feb.F != m("500") {
		t.Errorf("February F = %v, want 500 (asset 2's first deposit)", feb.F)
	}
	if feb.V != m("1510") || feb.D != m("1500") {
		t.Errorf("February V/D = %v/%v, want 1510/1500", feb.V, feb.D)
	}
	c := Chain(s.ChainPoints(), s.HasBaseline)
	if math.Abs(c.G[1]-(1510.0/1500.0)) > 1e-12 {
		t.Errorf("leg 2 = %.6f, want %.6f", c.G[1], 1510.0/1500.0)
	}
}

func TestDZeroGuard(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("0"), Value: m("0")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("100"), Value: m("110")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}})
	c := Chain(s.ChainPoints(), s.HasBaseline)
	if len(c.Months) != 1 || c.Months[0] != "2025-02" {
		t.Fatalf("legs = %v, want single 2025-02", c.Months)
	}
	if math.Abs(c.G[0]-1.1) > 1e-12 {
		t.Errorf("g = %.6f, want 1.1 (110/100)", c.G[0])
	}
	if len(c.Notes) == 0 {
		t.Error("expected a data-quality note for the D=0 guard")
	}
}

func TestEmptyThenRefilled(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("100"), Value: m("100")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("0"), Value: m("0")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("200"), Value: m("200")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}})
	c := Chain(s.ChainPoints(), s.HasBaseline)
	if len(c.Omitted) != 1 || c.Omitted[0] != 1 {
		t.Fatalf("Omitted = %v, want [1] (February leg)", c.Omitted)
	}
	if len(c.Notes) == 0 {
		t.Error("expected a data-quality note for the emptied-portfolio guard")
	}
	if math.Abs(c.TWRR-0.0) > 1e-12 {
		t.Errorf("TWRR = %.6f, want 0 (all factors 1)", c.TWRR)
	}
}

func TestRangeSliceBaseline(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2024-12"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1010")},
	}
	from := month("2025-01")
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}, From: &from})
	if !s.HasBaseline {
		t.Fatal("expected HasBaseline for mid-history range")
	}
	if len(s.Visible()) != 1 || !s.Visible()[0].Month.Equal(month("2025-01")) {
		t.Fatalf("visible = %+v, want 2025-01 only", s.Visible())
	}
	c := Chain(s.ChainPoints(), s.HasBaseline)
	if len(c.Months) != 1 || math.Abs(c.G[0]-1.01) > 1e-12 {
		t.Errorf("range TWRR leg = %v (%.6f), want single leg 1.01", c.Months, c.G[0])
	}
	if math.Abs(c.TWRR-0.01) > 1e-12 {
		t.Errorf("range TWRR = %.6f, want 0.01", c.TWRR)
	}
}

func TestClosedModesTotalsEqual(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1010")},
		{AssetID: 2, Month: month("2025-01"), Deposit: m("500"), Value: m("520")},
		{AssetID: 2, Month: month("2025-02"), Deposit: m("500"), Value: m("530")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1000"), Value: m("1020")},
	}
	assets := []domain.Asset{{ID: 1}, {ID: 2, Closed: true}}

	sAll := BuildSeries(Input{Rows: rows, Assets: assets, ClosedMode: ClosedAll})
	sMerged := BuildSeries(Input{Rows: rows, Assets: assets, ClosedMode: ClosedMerged})
	sActive := BuildSeries(Input{Rows: rows, Assets: assets, ClosedMode: ClosedActive})

	last := func(s Series) domain.Money {
		v := s.Visible()
		if len(v) == 0 {
			return 0
		}
		return v[len(v)-1].V
	}
	if last(sAll) != last(sMerged) {
		t.Errorf("merged total %v != all total %v", last(sMerged), last(sAll))
	}
	if last(sActive) != m("1020") {
		t.Errorf("active total = %v, want 1020 (closed asset dropped)", last(sActive))
	}
	if sActive.ClosedCount != 0 || sMerged.ClosedCount != 1 {
		t.Errorf("closed counts: active=%d merged=%d, want 0/1", sActive.ClosedCount, sMerged.ClosedCount)
	}
}

func TestClosedAssetSeriesStopsAtLastSnapshot(t *testing.T) {
	to := month("2025-06")
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("100"), Value: m("100")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1, Closed: true}}, To: &to})
	if len(s.Points) != 1 {
		t.Errorf("points = %d, want 1 (closed asset stops at last snapshot)", len(s.Points))
	}
}

func TestDecompositionIdentity(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1500"), Value: m("1520")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("1400"), Value: m("1440")},
		{AssetID: 1, Month: month("2025-04"), Deposit: m("1400"), Value: m("1490")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}})
	d := Decompose(s)
	// Jan is the seed month: with no prior month to differ from, the split
	// starts at Feb and measures V growth since the seed.
	vs := []float64{1520, 1440, 1490}
	if len(d.Months) != len(vs) || d.Months[0] != "2025-02" {
		t.Fatalf("decomposition months = %v, want Feb..Apr", d.Months)
	}
	prev := 1000.0
	for i := range d.Months {
		deltaV := vs[i] - prev
		if math.Abs((d.NewMoney[i]+d.Growth[i])-deltaV) > 1e-9 {
			t.Errorf("month %s: new+growth = %.2f, ΔV = %.2f", d.Months[i], d.NewMoney[i]+d.Growth[i], deltaV)
		}
		if math.Abs((d.CumNewMoney[i]+d.CumGrowth[i])-(vs[i]-1000.0)) > 1e-9 {
			t.Errorf("month %s: cumulative split = %.2f, V growth since seed = %.2f", d.Months[i], d.CumNewMoney[i]+d.CumGrowth[i], vs[i]-1000.0)
		}
		prev = vs[i]
	}
}

func TestDecompositionBaselineKeepsFirstVisibleMonth(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1500"), Value: m("1520")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("1500"), Value: m("1500")},
	}
	from := month("2025-02")
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}, From: &from})
	if !s.HasBaseline {
		t.Fatal("HasBaseline = false, want true")
	}
	d := Decompose(s)
	if len(d.Months) != 2 || d.Months[0] != "2025-02" {
		t.Fatalf("decomposition months = %v, want 2025-02..2025-03", d.Months)
	}
	// February diffs against the January baseline, so its 500 top-up is a
	// real month-over-month change and stays in the split.
	if math.Abs(d.NewMoney[0]-500.0) > 1e-9 {
		t.Errorf("new money[0] = %.2f, want 500", d.NewMoney[0])
	}
	if math.Abs(d.Growth[0]-20.0) > 1e-9 {
		t.Errorf("growth[0] = %.2f, want 20", d.Growth[0])
	}
}

func TestRiskStats(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1000"), Value: m("1020")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("1000"), Value: m("999.6")},
		{AssetID: 1, Month: month("2025-04"), Deposit: m("1000"), Value: m("1049.58")},
	}
	s := BuildSeries(Input{Rows: rows, Assets: []domain.Asset{{ID: 1}}})
	c := Chain(s.ChainPoints(), s.HasBaseline)
	h := HeadlineFrom(s, c)
	r := RiskFrom(s, c, h)

	if r.Best == nil || r.Best.Month != "2025-04" {
		t.Errorf("best month = %+v, want 2025-04 (+5%%)", r.Best)
	}
	if r.Worst == nil || r.Worst.Month != "2025-03" {
		t.Errorf("worst month = %+v, want 2025-03 (−2%%)", r.Worst)
	}
	if r.VolMonthly == nil {
		t.Fatal("volatility nil")
	}
	wantVol := 0.0351188
	if math.Abs(*r.VolMonthly-wantVol) > 1e-5 {
		t.Errorf("vol = %.7f, want ≈%.7f", *r.VolMonthly, wantVol)
	}
	wantDD := (1.02 - 0.9996) / 1.02
	if r.MaxDDGrowth == nil || math.Abs(r.MaxDDGrowth.Depth-wantDD) > 1e-9 {
		t.Errorf("growth drawdown = %+v, want %.6f trough 2025-03", r.MaxDDGrowth, wantDD)
	}
	if r.PositiveShare != 2.0/3.0 {
		t.Errorf("positive share = %.4f, want 2/3", r.PositiveShare)
	}
}

func TestTWRRSplitInvariance(t *testing.T) {
	rowsA := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1200"), Value: m("1212")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("1200"), Value: m("1224.12")},
	}
	rowsB := []domain.SeriesRow{
		{AssetID: 1, Month: month("2025-01"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2025-02"), Deposit: m("1100"), Value: m("1111")},
		{AssetID: 1, Month: month("2025-03"), Deposit: m("1200"), Value: m("1223.11")},
	}
	sA := BuildSeries(Input{Rows: rowsA, Assets: []domain.Asset{{ID: 1}}})
	sB := BuildSeries(Input{Rows: rowsB, Assets: []domain.Asset{{ID: 1}}})
	cA := Chain(sA.ChainPoints(), sA.HasBaseline)
	cB := Chain(sB.ChainPoints(), sB.HasBaseline)
	if math.Abs(cA.TWRR-cB.TWRR) > 1e-9 {
		t.Errorf("TWRR not split-invariant: A=%.8f B=%.8f", cA.TWRR, cB.TWRR)
	}
	if math.Abs(cA.TWRR-0.0201) > 1e-9 {
		t.Errorf("TWRR A = %.8f, want 0.0201 (1.01² − 1)", cA.TWRR)
	}
}

func TestYearlyTableOpeningBalance(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2020-10"), Deposit: m("5541.32"), Value: m("5541.45")},
		{AssetID: 1, Month: month("2020-11"), Deposit: m("5541.32"), Value: m("5544.06")},
		{AssetID: 1, Month: month("2020-12"), Deposit: m("5541.32"), Value: m("5546.61")},
	}
	assets := []domain.Asset{{ID: 1, ClassID: 3, Description: "Atom Bank Savings"}}
	s := BuildSeries(Input{Rows: rows, Assets: assets})
	ys := yearlyTable(compute(s))
	if len(ys) != 1 {
		t.Fatalf("years = %d, want 1", len(ys))
	}
	y := ys[0]
	if y["year"].(int) != 2020 {
		t.Fatalf("year = %v, want 2020", y["year"])
	}
	start, _ := y["start"].(float64)
	deposits, _ := y["deposits"].(float64)
	growth, _ := y["growth"].(float64)
	end, _ := y["end"].(float64)
	income, _ := y["income"].(float64)
	if math.Abs(start-0.13) > 0.011 {
		t.Errorf("start = %.2f, want ≈0.13 (V₀ − F₀)", start)
	}
	if math.Abs(deposits-5541.32) > 0.01 {
		t.Errorf("deposits = %.2f, want 5541.32", deposits)
	}
	if growth < 0 || growth > 10 {
		t.Errorf("growth = %.2f, want ≈5.16 (a few months of savings interest, not −5536)", growth)
	}
	if math.Abs(end-5546.61) > 0.01 {
		t.Errorf("end = %.2f, want 5546.61", end)
	}
	if math.Abs((start+deposits+growth-income)-end) > 0.02 {
		t.Errorf("identity start+deposits+growth−income = %.2f, want end %.2f", start+deposits+growth-income, end)
	}
	if y["change_pct"] != nil {
		t.Errorf("change_pct = %v, want nil for the inception year", y["change_pct"])
	}
}

func TestYearlyTableMidYearOpening(t *testing.T) {
	rows := []domain.SeriesRow{
		{AssetID: 1, Month: month("2020-12"), Deposit: m("1000"), Value: m("1000")},
		{AssetID: 1, Month: month("2021-01"), Deposit: m("1100"), Value: m("1105")},
		{AssetID: 1, Month: month("2021-12"), Deposit: m("1200"), Value: m("1215")},
	}
	assets := []domain.Asset{{ID: 1, ClassID: 1, Description: "Saver"}}
	s := BuildSeries(Input{Rows: rows, Assets: assets})
	ys := yearlyTable(compute(s))
	if len(ys) != 2 {
		t.Fatalf("years = %d, want 2", len(ys))
	}
	y := ys[0]
	if y["year"].(int) != 2021 {
		t.Fatalf("first row year = %v, want 2021", y["year"])
	}
	start, _ := y["start"].(float64)
	deposits, _ := y["deposits"].(float64)
	growth, _ := y["growth"].(float64)
	end, _ := y["end"].(float64)
	if math.Abs(start-1000) > 1e-9 {
		t.Errorf("start = %.2f, want 1000 (prior year-end close, not January's V)", start)
	}
	if math.Abs(deposits-200) > 1e-9 {
		t.Errorf("deposits = %.2f, want 200", deposits)
	}
	if math.Abs(growth-15) > 1e-9 {
		t.Errorf("growth = %.2f, want 15", growth)
	}
	if math.Abs(end-1215) > 1e-9 {
		t.Errorf("end = %.2f, want 1215", end)
	}
	if math.Abs((start+deposits+growth)-end) > 1e-9 {
		t.Errorf("identity start+deposits+growth = %.2f, want end %.2f", start+deposits+growth, end)
	}
}
