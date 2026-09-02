package importer

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"conspectus/internal/apiclient"
	"conspectus/internal/domain"
	"conspectus/internal/httpapi"
)

type fakeRepos struct {
	classes  map[int32]domain.Class
	assets   map[int32]domain.Asset
	logs     map[int32]domain.LogEntry
	payments map[int32]domain.Payment
	nextID   int32
}

func newFake() *fakeRepos {
	return &fakeRepos{
		classes:  map[int32]domain.Class{1: {ID: 1, Description: "Savings"}},
		assets:   map[int32]domain.Asset{1: {ID: 1, ClassID: 1, Description: "Chase Saver"}},
		logs:     map[int32]domain.LogEntry{},
		payments: map[int32]domain.Payment{},
	}
}

func (f *fakeRepos) Assets() domain.AssetRepo      { return &frAssets{f} }
func (f *fakeRepos) Classes() domain.ClassRepo     { return &frClasses{f} }
func (f *fakeRepos) Logs() domain.LogRepo          { return &frLogs{f} }
func (f *fakeRepos) Payments() domain.PaymentRepo  { return &frPayments{f} }
func (f *fakeRepos) Settings() domain.SettingsRepo { return nil }
func (f *fakeRepos) Tx(ctx context.Context, fn func(domain.Repos) error) error {
	return fn(f)
}
func (f *fakeRepos) Ping(ctx context.Context) error { return nil }
func (f *fakeRepos) Close() error                   { return nil }

type frAssets struct{ *fakeRepos }

func (f *frAssets) List(ctx context.Context, fl domain.AssetFilter) ([]domain.Asset, int, error) {
	var out []domain.Asset
	for _, a := range f.assets {
		out = append(out, a)
	}
	return out, len(out), nil
}

func (f *frAssets) Get(ctx context.Context, id int32) (domain.Asset, error) {
	a, ok := f.assets[id]
	if !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	return a, nil
}

func (f *frAssets) Create(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	f.nextID++
	a.ID = f.nextID
	f.assets[a.ID] = a
	return a, nil
}

func (f *frAssets) Update(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	f.assets[a.ID] = a
	return a, nil
}

func (f *frAssets) SetClosed(ctx context.Context, id int32, closed bool) error { return nil }

func (f *frAssets) Delete(ctx context.Context, id int32, force bool) (int64, int64, error) {
	return 0, 0, nil
}

type frClasses struct{ *fakeRepos }

func (f *frClasses) List(ctx context.Context) ([]domain.Class, error) {
	var out []domain.Class
	for _, c := range f.classes {
		out = append(out, c)
	}
	return out, nil
}
func (f *frClasses) Get(ctx context.Context, id int32) (domain.Class, error) {
	return domain.Class{}, domain.ErrNotFound
}
func (f *frClasses) Create(ctx context.Context, c domain.Class) (domain.Class, error) {
	f.nextID++
	c.ID = f.nextID
	f.classes[c.ID] = c
	return c, nil
}
func (f *frClasses) Update(ctx context.Context, c domain.Class) (domain.Class, error) {
	return c, nil
}
func (f *frClasses) Delete(ctx context.Context, id int32) error { return nil }

type frLogs struct{ *fakeRepos }

func (f *frLogs) List(ctx context.Context, fl domain.LogFilter) ([]domain.LogEntry, int, error) {
	var out []domain.LogEntry
	for _, e := range f.logs {
		if fl.AssetID != 0 && e.AssetID != fl.AssetID {
			continue
		}
		if fl.From != nil && domain.MonthOf(e.At, time.UTC).Before(*fl.From) {
			continue
		}
		if fl.To != nil && domain.MonthOf(e.At, time.UTC).After(*fl.To) {
			continue
		}
		out = append(out, e)
	}
	return out, len(out), nil
}
func (f *frLogs) Get(ctx context.Context, id int32) (domain.LogEntry, error) {
	return domain.LogEntry{}, domain.ErrNotFound
}
func (f *frLogs) Create(ctx context.Context, e domain.LogEntry) (domain.LogEntry, error) {
	for _, x := range f.logs {
		if x.AssetID == e.AssetID && x.At.Year() == e.At.Year() && x.At.Month() == e.At.Month() {
			return domain.LogEntry{}, &domain.ConflictError{Message: "dup", ExistingID: x.ID}
		}
	}
	f.nextID++
	e.ID = f.nextID
	f.logs[e.ID] = e
	return e, nil
}
func (f *frLogs) Update(ctx context.Context, e domain.LogEntry) (domain.LogEntry, error) {
	f.logs[e.ID] = e
	return e, nil
}
func (f *frLogs) Delete(ctx context.Context, id int32) error { return nil }
func (f *frLogs) FindByAssetMonth(ctx context.Context, assetID int32, m domain.Month) (domain.LogEntry, error) {
	for _, x := range f.logs {
		if x.AssetID == assetID && domain.MonthOf(x.At, time.UTC).Equal(m) {
			return x, nil
		}
	}
	return domain.LogEntry{}, domain.ErrNotFound
}
func (f *frLogs) SeriesRows(ctx context.Context, q domain.SeriesQuery) ([]domain.SeriesRow, error) {
	return nil, nil
}

type frPayments struct{ *fakeRepos }

func (f *frPayments) List(ctx context.Context, fl domain.PaymentFilter) ([]domain.Payment, int, error) {
	return nil, 0, nil
}
func (f *frPayments) Get(ctx context.Context, id int32) (domain.Payment, error) {
	return domain.Payment{}, domain.ErrNotFound
}
func (f *frPayments) Create(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	f.nextID++
	p.ID = f.nextID
	f.payments[p.ID] = p
	return p, nil
}
func (f *frPayments) Update(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	return p, nil
}
func (f *frPayments) Delete(ctx context.Context, id int32) error { return nil }

func newTestImporter(store *fakeRepos) *Importer {
	router := httpapi.NewRouter()
	api := &httpapi.API{
		Router: router,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Repos:  func() (domain.Repos, error) { return store, nil },
	}
	if err := api.RegisterCoreRoutes(); err != nil {
		panic(err)
	}
	return New(apiclient.NewLoopback(router), time.UTC)
}

func csvLines(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func TestCSVPreviewValidation(t *testing.T) {
	store := newFake()
	closed := domain.Asset{ID: 2, ClassID: 1, Description: "Old ISA", Closed: true}
	store.assets[2] = closed
	thisMonth := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	seedDep, _ := domain.ParseMoney("1000.00")
	store.logs[9] = domain.LogEntry{ID: 9, AssetID: 1, At: thisMonth, Deposit: seedDep}
	imp := newTestImporter(store)
	aug30 := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	imp.Now = func() time.Time { return aug30 }

	report, token, err := imp.Preview(context.Background(), strings.NewReader(csvLines(
		"id,name,deposit,value",
		"1,Chase Saver,1500.00,1520.00",
		"2,Old ISA,500.00,510.00",
		"99,Ghost,1.00,1.00",
		",no id here,1.00,2.00",
		"3,Future Asset,1.00,2.00",
		"1,Chase Saver,abc,2.00",
	)), aug30)
	if err != nil {
		t.Fatal(err)
	}
	if report.Creates != 0 || report.Updates != 1 || report.Skipped != 2 || report.Errors != 1 || report.Closed != 1 || report.Unknown != 2 {
		t.Fatalf("report = create:%d update:%d skip:%d err:%d closed:%d unknown:%d",
			report.Creates, report.Updates, report.Skipped, report.Errors, report.Closed, report.Unknown)
	}
	for _, row := range report.Rows {
		if row.Line == 5 && row.AssetName != "no id here" {
			t.Fatalf("skipped no-id row shows name %q, want %q", row.AssetName, "no id here")
		}
	}
	if token == "" {
		t.Fatal("no preview token")
	}
	if len(report.Warnings()) != 2 {
		t.Fatalf("warnings = %v", report.Warnings())
	}

	res, err := imp.Commit(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || res.Created != 0 {
		t.Fatalf("commit = %+v", res)
	}
	if store.logs[9].Deposit.String() != "1500.00" {
		t.Fatalf("updated entry = %+v", store.logs[9])
	}
	if _, err := imp.Commit(context.Background(), token); err == nil {
		t.Fatal("token reuse allowed")
	}
}

func TestCSVCommitCreatesAndWarnsOnZero(t *testing.T) {
	store := newFake()
	imp := newTestImporter(store)
	aug30 := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	imp.Now = func() time.Time { return aug30 }

	report, token, err := imp.Preview(context.Background(), strings.NewReader(csvLines(
		"id,name,deposit,value",
		"1,Chase Saver,100.00,104.00",
	)), aug30)
	if err != nil {
		t.Fatal(err)
	}
	if report.Creates != 1 || report.Updates != 0 {
		t.Fatalf("report = %+v", report)
	}
	res, err := imp.Commit(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || len(res.Warnings) != 0 {
		t.Fatalf("commit = %+v", res)
	}
	var e domain.LogEntry
	for _, x := range store.logs {
		e = x
	}
	if e.AssetID != 1 || e.At.Month() != time.August || e.At.Year() != 2026 || e.Value.String() != "104.00" {
		t.Fatalf("created entry = %+v", e)
	}

	_, token2, err := imp.Preview(context.Background(), strings.NewReader(csvLines(
		"id,name,deposit,value",
		"77,Ghost,1.00,1.00",
	)), aug30)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := imp.Commit(context.Background(), token2)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Created+res2.Updated != 0 || len(res2.Warnings) != 1 {
		t.Fatalf("commit2 = %+v", res2)
	}
}

func TestCSVEmptyOrJunkFile(t *testing.T) {
	imp := newTestImporter(newFake())
	aug30 := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	imp.Now = func() time.Time { return aug30 }
	if _, _, err := imp.Preview(context.Background(), strings.NewReader(""), aug30); err == nil {
		t.Fatal("expected error on empty file")
	}
	if _, _, err := imp.Preview(context.Background(), strings.NewReader("\"unterminated\nrow\n"), aug30); err == nil {
		t.Fatal("expected CSV parse error")
	}
}

func TestCSVBackMonthAttribution(t *testing.T) {
	store := newFake()
	augDep, _ := domain.ParseMoney("1000.00")
	store.logs[9] = domain.LogEntry{ID: 9, AssetID: 1, At: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), Deposit: augDep}
	imp := newTestImporter(store)
	imp.Now = func() time.Time { return time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC) }

	// July file uploaded in August: lands in July, leaves the August entry alone.
	jul15 := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	report, token, err := imp.Preview(context.Background(), strings.NewReader(csvLines(
		"id,name,deposit,value",
		"1,Chase Saver,900.00,910.00",
	)), jul15)
	if err != nil {
		t.Fatal(err)
	}
	if report.Creates != 1 || report.Updates != 0 {
		t.Fatalf("back-month report = %+v", report)
	}
	res, err := imp.Commit(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || res.Updated != 0 {
		t.Fatalf("back-month commit = %+v", res)
	}
	var julyEntry, augustEntry *domain.LogEntry
	for i, x := range store.logs {
		e := store.logs[i]
		if x.At.Year() == 2026 && x.At.Month() == time.July {
			julyEntry = &e
		}
		if x.ID == 9 {
			augustEntry = &e
		}
	}
	if julyEntry == nil {
		t.Fatal("no July entry written")
	}
	if augustEntry.Deposit.String() != "1000.00" {
		t.Fatalf("August entry disturbed: %+v", *augustEntry)
	}
}

func TestQuickSubmissionParsing(t *testing.T) {
	form := map[string][]string{
		"date":      {"2026-08-01"},
		"deposit_1": {"100.00"},
		"value_1":   {"101.00"},
		"payment_1": {"2.00"},
		"deposit_2": {"50"},
		"value_2":   {"51.5"},
		"skip_3":    {"on"},
		"deposit_3": {"1.00"},
		"value_3":   {"1.00"},
	}
	entries, err := ParseQuickSubmission(form)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (id 3 skipped)", len(entries))
	}
	if entries[0].AssetID != 1 || entries[0].Deposit.String() != "100.00" || entries[0].Payment.String() != "2.00" {
		t.Errorf("entry 1 = %+v", entries[0])
	}
	if entries[1].Value.String() != "51.50" {
		t.Errorf("entry 2 = %+v", entries[1])
	}
}
