package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"conspectus/internal/domain"
	"conspectus/internal/settings"
	"conspectus/internal/stats"
)

type fakeStore struct {
	classes             map[int32]domain.Class
	assets              map[int32]domain.Asset
	logs                map[int32]domain.LogEntry
	payments            map[int32]domain.Payment
	settings             map[string]string
	settingsDescriptions map[string]string
	settingsDisplay      map[string]bool
	nextID               int32
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		classes:              map[int32]domain.Class{1: {ID: 1, Description: "Savings"}},
		assets:               map[int32]domain.Asset{},
		logs:                 map[int32]domain.LogEntry{},
		payments:             map[int32]domain.Payment{},
		settings:             map[string]string{},
		settingsDescriptions: map[string]string{},
		settingsDisplay:      map[string]bool{},
	}
}

func (f *fakeStore) Assets() domain.AssetRepo      { return &fakeAssets{f} }
func (f *fakeStore) Classes() domain.ClassRepo     { return &fakeClasses{f} }
func (f *fakeStore) Logs() domain.LogRepo          { return &fakeLogs{f} }
func (f *fakeStore) Payments() domain.PaymentRepo  { return &fakePayments{f} }
func (f *fakeStore) Settings() domain.SettingsRepo { return &fakeSettings{f} }
func (f *fakeStore) Tx(ctx context.Context, fn func(domain.Repos) error) error {
	return fn(f)
}
func (f *fakeStore) Ping(ctx context.Context) error { return nil }
func (f *fakeStore) Close() error                   { return nil }

type fakeAssets struct{ *fakeStore }

func (f *fakeAssets) List(ctx context.Context, fl domain.AssetFilter) ([]domain.Asset, int, error) {
	var out []domain.Asset
	for _, a := range f.assets {
		if fl.ClassID != 0 && a.ClassID != fl.ClassID {
			continue
		}
		if fl.Query != "" && !strings.Contains(strings.ToLower(a.Description), strings.ToLower(fl.Query)) {
			continue
		}
		out = append(out, a)
	}
	return out, len(out), nil
}

func (f *fakeAssets) Get(ctx context.Context, id int32) (domain.Asset, error) {
	a, ok := f.assets[id]
	if !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	return a, nil
}

func (f *fakeAssets) Create(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	if _, ok := f.classes[a.ClassID]; !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	for _, existing := range f.assets {
		if existing.ClassID == a.ClassID && strings.EqualFold(existing.Description, a.Description) {
			return domain.Asset{}, fmt.Errorf("%w: an asset with this description already exists in the class", domain.ErrConflict)
		}
	}
	f.nextID++
	a.ID = f.nextID
	f.assets[a.ID] = a
	return a, nil
}

func (f *fakeAssets) Update(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	if _, ok := f.assets[a.ID]; !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	f.assets[a.ID] = a
	return a, nil
}

func (f *fakeAssets) SetClosed(ctx context.Context, id int32, closed bool) error {
	a, ok := f.assets[id]
	if !ok {
		return domain.ErrNotFound
	}
	a.Closed = closed
	f.assets[id] = a
	return nil
}

func (f *fakeAssets) Delete(ctx context.Context, id int32, force bool) (int64, int64, error) {
	var logs, pays int64
	for _, e := range f.logs {
		if e.AssetID == id {
			logs++
		}
	}
	for _, p := range f.payments {
		if p.AssetID == id {
			pays++
		}
	}
	if (logs > 0 || pays > 0) && !force {
		return logs, pays, fmt.Errorf("%w: asset has rows", domain.ErrConflict)
	}
	delete(f.assets, id)
	return logs, pays, nil
}

type fakeClasses struct{ *fakeStore }

func (f *fakeClasses) List(ctx context.Context) ([]domain.Class, error) {
	var out []domain.Class
	for _, c := range f.classes {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeClasses) Get(ctx context.Context, id int32) (domain.Class, error) {
	c, ok := f.classes[id]
	if !ok {
		return domain.Class{}, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeClasses) Create(ctx context.Context, c domain.Class) (domain.Class, error) {
	for _, existing := range f.classes {
		if strings.EqualFold(existing.Description, c.Description) {
			return domain.Class{}, fmt.Errorf("%w: duplicate class", domain.ErrConflict)
		}
	}
	f.nextID++
	c.ID = f.nextID
	f.classes[c.ID] = c
	return c, nil
}

func (f *fakeClasses) Update(ctx context.Context, c domain.Class) (domain.Class, error) {
	if _, ok := f.classes[c.ID]; !ok {
		return domain.Class{}, domain.ErrNotFound
	}
	f.classes[c.ID] = c
	return c, nil
}

func (f *fakeClasses) Delete(ctx context.Context, id int32) error {
	for _, a := range f.assets {
		if a.ClassID == id {
			return fmt.Errorf("%w: assets reference class", domain.ErrConflict)
		}
	}
	delete(f.classes, id)
	return nil
}

type fakeLogs struct{ *fakeStore }

func (f *fakeLogs) List(ctx context.Context, fl domain.LogFilter) ([]domain.LogEntry, int, error) {
	var out []domain.LogEntry
	for _, e := range f.logs {
		if fl.AssetID != 0 && e.AssetID != fl.AssetID {
			continue
		}
		out = append(out, e)
	}
	return out, len(out), nil
}

func (f *fakeLogs) Get(ctx context.Context, id int32) (domain.LogEntry, error) {
	e, ok := f.logs[id]
	if !ok {
		return domain.LogEntry{}, domain.ErrNotFound
	}
	return e, nil
}

func (f *fakeLogs) Create(ctx context.Context, e domain.LogEntry) (domain.LogEntry, error) {
	if _, ok := f.assets[e.AssetID]; !ok {
		return domain.LogEntry{}, domain.ErrNotFound
	}
	for _, existing := range f.logs {
		if existing.AssetID == e.AssetID && existing.At.Year() == e.At.Year() && existing.At.Month() == e.At.Month() {
			return domain.LogEntry{}, &domain.ConflictError{Message: "duplicate month", ExistingID: existing.ID}
		}
	}
	f.nextID++
	e.ID = f.nextID
	f.logs[e.ID] = e
	return e, nil
}

func (f *fakeLogs) Update(ctx context.Context, e domain.LogEntry) (domain.LogEntry, error) {
	if _, ok := f.logs[e.ID]; !ok {
		return domain.LogEntry{}, domain.ErrNotFound
	}
	f.logs[e.ID] = e
	return e, nil
}

func (f *fakeLogs) Delete(ctx context.Context, id int32) error {
	if _, ok := f.logs[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.logs, id)
	return nil
}

func (f *fakeLogs) FindByAssetMonth(ctx context.Context, assetID int32, m domain.Month) (domain.LogEntry, error) {
	for _, e := range f.logs {
		em := domain.MonthOf(e.At, time.UTC)
		if e.AssetID == assetID && em.Equal(m) {
			return e, nil
		}
	}
	return domain.LogEntry{}, domain.ErrNotFound
}

func (f *fakeLogs) SeriesRows(ctx context.Context, q domain.SeriesQuery) ([]domain.SeriesRow, error) {
	return nil, nil
}

type fakePayments struct{ *fakeStore }

func (f *fakePayments) List(ctx context.Context, fl domain.PaymentFilter) ([]domain.Payment, int, error) {
	var out []domain.Payment
	for _, p := range f.payments {
		if fl.AssetID != 0 && p.AssetID != fl.AssetID {
			continue
		}
		out = append(out, p)
	}
	return out, len(out), nil
}

func (f *fakePayments) Get(ctx context.Context, id int32) (domain.Payment, error) {
	p, ok := f.payments[id]
	if !ok {
		return domain.Payment{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakePayments) Create(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	if _, ok := f.assets[p.AssetID]; !ok {
		return domain.Payment{}, domain.ErrNotFound
	}
	f.nextID++
	p.ID = f.nextID
	f.payments[p.ID] = p
	return p, nil
}

func (f *fakePayments) Update(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	if _, ok := f.payments[p.ID]; !ok {
		return domain.Payment{}, domain.ErrNotFound
	}
	f.payments[p.ID] = p
	return p, nil
}

func (f *fakePayments) Delete(ctx context.Context, id int32) error {
	delete(f.payments, id)
	return nil
}

type fakeSettings struct{ *fakeStore }

func (f *fakeSettings) Get(ctx context.Context, key string) (string, error) {
	v, ok := f.settings[key]
	if !ok {
		return "", domain.ErrNotFound
	}
	return v, nil
}

func (f *fakeSettings) Set(ctx context.Context, key, value string) error {
	f.settings[key] = value
	return nil
}

func (f *fakeSettings) Create(ctx context.Context, key, value, description string, display bool) error {
	if _, ok := f.settings[key]; ok {
		return domain.ErrConflict
	}
	f.settings[key] = value
	f.settingsDescriptions[key] = description
	f.settingsDisplay[key] = display
	return nil
}

func (f *fakeSettings) SetDescription(ctx context.Context, key, description string) error {
	if _, ok := f.settings[key]; !ok {
		f.settings[key] = ""
	}
	f.settingsDescriptions[key] = description
	return nil
}

func (f *fakeSettings) SetDisplay(ctx context.Context, key string, display bool) error {
	if display {
		if _, ok := f.settings[key]; !ok {
			f.settings[key] = ""
		}
	}
	f.settingsDisplay[key] = display
	return nil
}

func (f *fakeSettings) Delete(ctx context.Context, key string) error {
	delete(f.settings, key)
	delete(f.settingsDescriptions, key)
	delete(f.settingsDisplay, key)
	return nil
}

func (f *fakeSettings) All(ctx context.Context) (map[string]string, error) {
	return f.settings, nil
}

func (f *fakeSettings) List(ctx context.Context) ([]domain.Setting, error) {
	out := []domain.Setting{}
	for k, v := range f.settings {
		out = append(out, domain.Setting{
			Key: k, Value: v,
			Description: f.settingsDescriptions[k],
			Display:     f.settingsDisplay[k],
		})
	}
	return out, nil
}

func newTestAPI(t *testing.T) (*httpapiServer, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	router := NewRouter()
	log := slog.New(slog.DiscardHandler)
	set := settings.New(log, func() (domain.SettingsRepo, error) { return store.Settings(), nil })
	InstallCoreMiddleware(router, MiddlewareDeps{Log: log, Settings: set})
	api := &API{
		Router: router, Settings: set, Log: log,
		Loc:   time.UTC,
		Info:  SystemInfo{Version: "test", ExpectedSchemaVersion: 2},
		Repos: func() (domain.Repos, error) { return store, nil },
		Now:   func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) },
		Stats: &stats.Service{
			Repos:    func() (domain.Repos, error) { return store, nil },
			Settings: set,
			Log:      log,
			Now:      func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) },
		},
	}
	if err := api.RegisterCoreRoutes(); err != nil {
		t.Fatal(err)
	}
	return &httpapiServer{router}, store
}

type httpapiServer struct{ r *Router }

func (s *httpapiServer) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = strings.NewReader(string(data))
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	s.r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestAssetsCRUDFlow(t *testing.T) {
	srv, store := newTestAPI(t)

	rec := srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "Chase Saver"})
	if rec.Code != 201 {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	rec = srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "chase saver"})
	if rec.Code != 409 {
		t.Fatalf("duplicate = %d", rec.Code)
	}
	rec = srv.do(t, "GET", "/api/v1/assets/1", nil)
	if rec.Code != 200 {
		t.Fatalf("get = %d", rec.Code)
	}
	rec = srv.do(t, "PATCH", "/api/v1/assets/1", map[string]any{"description": "Chase Saver 2"})
	if rec.Code != 200 {
		t.Fatalf("patch = %d", rec.Code)
	}
	if got := store.assets[1].Description; got != "Chase Saver 2" {
		t.Errorf("description = %q", got)
	}
	srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date":    "2026-07-01",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "100.00", "value": "101.00"}},
	})
	rec = srv.do(t, "DELETE", "/api/v1/assets/1", nil)
	if rec.Code != 409 {
		t.Fatalf("delete blocked = %d: %s", rec.Code, rec.Body.String())
	}
	rec = srv.do(t, "DELETE", "/api/v1/assets/1?force=true", nil)
	if rec.Code != 200 {
		t.Fatalf("force delete = %d", rec.Code)
	}
}

func TestLogValidationAndDuplicates(t *testing.T) {
	srv, _ := newTestAPI(t)
	srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "A"})

	rec := srv.do(t, "POST", "/api/v1/logs", map[string]any{"date": "2026-07-01", "entries": []map[string]any{}})
	if rec.Code != 400 {
		t.Fatalf("empty entries = %d", rec.Code)
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date":    "2026-07-01",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "100", "value": "101"}},
	})
	if rec.Code != 200 {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date":    "2026-07-20",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "100", "value": "101"}},
	})
	if rec.Code != 200 {
		t.Fatalf("dup month = %d: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	data := body["data"].(map[string]any)
	if data["written"].(float64) != 0 {
		t.Errorf("dup written = %v", data["written"])
	}
	dups := data["duplicates"].([]any)
	if len(dups) != 1 {
		t.Fatalf("duplicates = %v", dups)
	}
	dup := dups[0].(map[string]any)
	if dup["action"] != "skipped" || dup["existing_id"].(float64) != 2 {
		t.Errorf("duplicate = %v", dup)
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date":    "2027-01-01",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "1", "value": "1"}},
	})
	if rec.Code != 400 {
		t.Fatalf("future = %d", rec.Code)
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date":    "2026-08-01",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "1.234", "value": "1"}},
	})
	if rec.Code != 400 {
		t.Fatalf("money 3dp = %d", rec.Code)
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date": "2026-08-01", "on_duplicate": "delete",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "1", "value": "1"}},
	})
	if rec.Code != 400 {
		t.Fatalf("bad on_duplicate = %d", rec.Code)
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date": "2026-08-01",
		"entries": []map[string]any{
			{"asset_id": 1, "deposit": "1", "value": "1"},
			{"asset_id": 1, "deposit": "2", "value": "2"},
		},
	})
	if rec.Code != 400 {
		t.Fatalf("duplicate asset in entries = %d", rec.Code)
	}
}

func TestRangeParamsValidateAsMonths(t *testing.T) {
	srv, _ := newTestAPI(t)
	srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "A"})

	for _, p := range []string{
		"/api/v1/stats/overview?to=abc",
		"/api/v1/stats/overview?from=nope",
		"/api/v1/stats/series?metric=value&to=2026-13",
		"/api/v1/stats/income?to=abc",
		"/api/v1/stats/breakdown?from=abc",
		"/api/v1/stats/analytics?to=abc",
		"/api/v1/stats/projections?from=abc",
		"/api/v1/logs?to=abc",
		"/api/v1/logs?from=2026-13",
		"/api/v1/payments?to=abc",
	} {
		rec := srv.do(t, "GET", p, nil)
		if rec.Code != 400 {
			t.Errorf("%s = %d, want 400 for a non-month range", p, rec.Code)
		}
	}

	rec := srv.do(t, "GET", "/api/v1/stats/overview?from=2026-01&to=2026-08", nil)
	if rec.Code != 200 {
		t.Errorf("valid range = %d: %s", rec.Code, rec.Body.String())
	}
	rec = srv.do(t, "GET", "/api/v1/logs?from=2026-01&to=2026-08", nil)
	if rec.Code != 200 {
		t.Errorf("valid logs range = %d: %s", rec.Code, rec.Body.String())
	}
	rec = srv.do(t, "GET", "/api/v1/stats/series?metric=nonsense", nil)
	if rec.Code != 400 {
		t.Errorf("unknown metric = %d, want 400", rec.Code)
	}
}

func TestLogsMonthSnapshot(t *testing.T) {
	srv, store := newTestAPI(t)
	rec := srv.do(t, "POST", "/api/v1/logs/bulk", map[string]any{"date": "2026-08-01", "entries": []map[string]any{}})
	if rec.Code != 404 {
		t.Fatalf("removed bulk route = %d", rec.Code)
	}
	srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "A"})
	srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "B"})

	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date": "2026-08-01",
		"entries": []map[string]any{
			{"asset_id": 1, "deposit": "100.00", "value": "101.00", "payment": "2.00"},
			{"asset_id": 2, "deposit": "50.00", "value": "50.00"},
		},
	})
	if rec.Code != 200 {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	data := body["data"].(map[string]any)
	if data["written"].(float64) != 2 {
		t.Errorf("written = %v", data["written"])
	}
	if data["payments_written"].(float64) != 1 {
		t.Errorf("payments = %v", data["payments_written"])
	}

	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date":    "2026-08-01",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "100.00", "value": "999.00"}},
	})
	body = decode(t, rec)
	data = body["data"].(map[string]any)
	if data["written"].(float64) != 0 {
		t.Errorf("skip written = %v", data["written"])
	}
	dups := data["duplicates"].([]any)
	if len(dups) != 1 {
		t.Fatalf("duplicates = %v", dups)
	}
	rec = srv.do(t, "POST", "/api/v1/logs", map[string]any{
		"date": "2026-08-01", "on_duplicate": "replace",
		"entries": []map[string]any{{"asset_id": 1, "deposit": "100.00", "value": "999.00"}},
	})
	body = decode(t, rec)
	data = body["data"].(map[string]any)
	if data["replaced"].(float64) != 1 {
		t.Errorf("replaced = %v", data["replaced"])
	}
	for _, e := range store.logs {
		if e.AssetID == 1 && e.At.Month() == time.August && e.Value.String() != "999.00" {
			t.Errorf("replace did not apply: %v", e)
		}
	}
}

func TestPaymentsCRUDFlow(t *testing.T) {
	srv, store := newTestAPI(t)
	srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "A"})

	rec := srv.do(t, "POST", "/api/v1/payments", map[string]any{"asset_id": 1, "date": "2026-07-15", "amount": "41.20"})
	if rec.Code != 201 {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	payID := int(decode(t, rec)["data"].(map[string]any)["id"].(float64))
	idPath := fmt.Sprintf("/api/v1/payments/%d", payID)
	rec = srv.do(t, "POST", "/api/v1/payments", map[string]any{"asset_id": 1, "date": "2026-07-15"})
	if rec.Code != 400 {
		t.Fatalf("missing amount = %d", rec.Code)
	}
	rec = srv.do(t, "POST", "/api/v1/payments", map[string]any{"asset_id": 1, "date": "2027-01-01", "amount": "1.00"})
	if rec.Code != 400 {
		t.Fatalf("future = %d", rec.Code)
	}
	rec = srv.do(t, "GET", "/api/v1/payments?asset_id=1", nil)
	if rec.Code != 200 {
		t.Fatalf("list = %d", rec.Code)
	}
	body := decode(t, rec)
	if data := body["data"].([]any); len(data) != 1 {
		t.Fatalf("list data = %v", data)
	}
	rec = srv.do(t, "PATCH", idPath, map[string]any{"amount": "42.00", "date": "2026-07-20"})
	if rec.Code != 200 {
		t.Fatalf("patch = %d: %s", rec.Code, rec.Body.String())
	}
	rec = srv.do(t, "DELETE", idPath, nil)
	if rec.Code != 200 {
		t.Fatalf("delete = %d", rec.Code)
	}
	if len(store.payments) != 0 {
		t.Fatalf("payments after delete = %v", store.payments)
	}
	rec = srv.do(t, "GET", idPath, nil)
	if rec.Code != 404 {
		t.Fatalf("get after delete = %d", rec.Code)
	}
}

func TestReadOnlyAndCSRF(t *testing.T) {
	srv, store := newTestAPI(t)
	srv.do(t, "POST", "/api/v1/assets", map[string]any{"class_id": 1, "description": "A"})

	// A stored read_only row is inert (the mode is env-only) and the key
	// is not guarded: patching it just writes a dead value.
	store.settings["read_only"] = "1"
	rec := srv.do(t, "PATCH", "/api/v1/settings/read_only", map[string]any{"value": "0"})
	if rec.Code != 200 {
		t.Fatalf("read_only patch = %d, want 200 (key not guarded)", rec.Code)
	}
	rec = srv.do(t, "POST", "/api/v1/classes", map[string]any{"description": "X"})
	if rec.Code != 201 {
		t.Fatalf("stale read_only row must not latch (env-var only now): %d", rec.Code)
	}

	req := httptest.NewRequest("POST", "/api/v1/classes", strings.NewReader(`{"description":"Y"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "conspectus_csrf", Value: "tok123"})
	rec = httptest.NewRecorder()
	srv.r.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("csrf missing = %d", rec.Code)
	}
	req2 := httptest.NewRequest("POST", "/api/v1/classes", strings.NewReader(`{"description":"Y"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-CSRF-Token", "tok123")
	req2.AddCookie(&http.Cookie{Name: "conspectus_csrf", Value: "tok123"})
	rec2 := httptest.NewRecorder()
	srv.r.ServeHTTP(rec2, req2)
	if rec2.Code != 201 {
		t.Fatalf("csrf ok = %d", rec2.Code)
	}

	req3 := httptest.NewRequest("POST", "/api/v1/classes", strings.NewReader(`{}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Origin", "https://evil.example")
	req3.AddCookie(&http.Cookie{Name: "conspectus_csrf", Value: "tok123"})
	req3.Header.Set("X-CSRF-Token", "tok123")
	rec3 := httptest.NewRecorder()
	srv.r.ServeHTTP(rec3, req3)
	if rec3.Code != 403 {
		t.Fatalf("cross-origin = %d", rec3.Code)
	}
}

func TestSettingsRedaction(t *testing.T) {
	srv, store := newTestAPI(t)
	// A secret key that is not auth config (basicauth_* now activates the
	// core auth gate, which would challenge this request).
	store.settings["webhook_secret"] = "deadbeef"
	store.settingsDescriptions["webhook_secret"] = "Hook"
	store.settingsDisplay["webhook_secret"] = true
	store.settings["currency"] = "GBP"
	rec := srv.do(t, "GET", "/api/v1/settings", nil)
	if rec.Code != 200 {
		t.Fatalf("settings = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "(redacted)") {
		t.Errorf("secret not redacted: %s", body)
	}
	if strings.Contains(body, "deadbeef") {
		t.Errorf("secret leaked: %s", body)
	}
	var parsed struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response is not a row array: %v", err)
	}
	byKey := map[string]map[string]any{}
	for _, row := range parsed.Data {
		byKey[row["key"].(string)] = row
	}
	row := byKey["webhook_secret"]
	if row == nil {
		t.Fatal("webhook_secret missing from settings list")
	}
	for _, field := range []string{"value", "description", "display"} {
		if _, ok := row[field]; !ok {
			t.Errorf("row missing %q field: %v", field, row)
		}
	}
	if row["value"] != "(redacted)" || row["description"] != "Hook" || row["display"] != true {
		t.Errorf("webhook_secret row = %v", row)
	}
	if byKey["currency"]["value"] != "GBP" {
		t.Errorf("currency row = %v", byKey["currency"])
	}
}

func TestSettingCreateAndUpdateOnlyExisting(t *testing.T) {
	srv, store := newTestAPI(t)

	// PATCH is update-only: unknown keys are refused, not created.
	rec := srv.do(t, "PATCH", "/api/v1/settings/fresh", map[string]any{"value": "1"})
	if rec.Code != 404 {
		t.Fatalf("patch unknown key = %d, want 404", rec.Code)
	}
	if _, ok := store.settings["fresh"]; ok {
		t.Fatal("patch created a row")
	}

	// POST creates the row with value, description and display in one go.
	rec = srv.do(t, "POST", "/api/v1/settings/club_note", map[string]any{
		"value": "hello", "description": "Club note", "display": true,
	})
	if rec.Code != 201 {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	if store.settings["club_note"] != "hello" ||
		store.settingsDescriptions["club_note"] != "Club note" ||
		!store.settingsDisplay["club_note"] {
		t.Fatalf("created row = %q / %q / %v",
			store.settings["club_note"], store.settingsDescriptions["club_note"], store.settingsDisplay["club_note"])
	}
	if !strings.Contains(rec.Body.String(), `"value":"hello"`) ||
		!strings.Contains(rec.Body.String(), `"display":true`) {
		t.Fatalf("create should echo the row: %s", rec.Body.String())
	}

	// Recreating an existing key is a conflict.
	if rec = srv.do(t, "POST", "/api/v1/settings/club_note", map[string]any{"value": "again"}); rec.Code != 409 {
		t.Fatalf("duplicate create = %d, want 409", rec.Code)
	}

	// Keys over 20 characters are refused.
	if rec = srv.do(t, "POST", "/api/v1/settings/this_key_is_way_too_long", map[string]any{"value": "x"}); rec.Code != 400 {
		t.Fatalf("long key create = %d, want 400", rec.Code)
	}

	// PATCH updates the created row's value and nothing else.
	rec = srv.do(t, "PATCH", "/api/v1/settings/club_note", map[string]any{"value": "changed"})
	if rec.Code != 200 {
		t.Fatalf("patch existing = %d: %s", rec.Code, rec.Body.String())
	}
	if store.settings["club_note"] != "changed" ||
		store.settingsDescriptions["club_note"] != "Club note" ||
		!store.settingsDisplay["club_note"] {
		t.Fatal("patch clobbered description or display flag")
	}

	// Migration-runner keys are refused on both verbs.
	if rec = srv.do(t, "POST", "/api/v1/settings/db_version", map[string]any{"value": "99"}); rec.Code != 400 {
		t.Fatalf("db_version create = %d, want 400", rec.Code)
	}
	if rec = srv.do(t, "PATCH", "/api/v1/settings/db_version", map[string]any{"value": "99"}); rec.Code != 400 {
		t.Fatalf("db_version patch = %d, want 400", rec.Code)
	}
}

func TestSettingPatchAndDelete(t *testing.T) {
	srv, store := newTestAPI(t)

	// The collection POST/PATCH and the display subresource are gone.
	if rec := srv.do(t, "POST", "/api/v1/settings", map[string]any{"key": "note"}); rec.Code != 404 {
		t.Fatalf("collection post = %d, want 404", rec.Code)
	}
	if rec := srv.do(t, "PATCH", "/api/v1/settings", map[string]any{"note": "x"}); rec.Code != 404 {
		t.Fatalf("collection patch = %d, want 404", rec.Code)
	}
	if rec := srv.do(t, "GET", "/api/v1/settings/display", nil); rec.Code != 404 {
		t.Fatalf("display list = %d, want 404", rec.Code)
	}
	if rec := srv.do(t, "PATCH", "/api/v1/settings/display/note", map[string]any{"display": true}); rec.Code != 404 {
		t.Fatalf("display patch = %d, want 404", rec.Code)
	}
	if rec := srv.do(t, "DELETE", "/api/v1/settings/display/note", nil); rec.Code != 404 {
		t.Fatalf("display-path delete = %d, want 404", rec.Code)
	}

	// Patching an absent key does not create it.
	if rec := srv.do(t, "PATCH", "/api/v1/settings/note", map[string]any{"value": "x"}); rec.Code != 404 {
		t.Fatalf("patch absent key = %d, want 404", rec.Code)
	}
	if _, ok := store.settings["note"]; ok {
		t.Fatal("patch created a row")
	}

	// POST creates with metadata only; an empty value is fine.
	rec := srv.do(t, "POST", "/api/v1/settings/note", map[string]any{"description": "A note", "display": true})
	if rec.Code != 201 {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	if v, ok := store.settings["note"]; !ok || v != "" {
		t.Fatalf("create should default to an empty value, got %q (exists=%v)", v, ok)
	}

	// The row and its fields show up in the collection GET.
	rec = srv.do(t, "GET", "/api/v1/settings", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"description":"A note"`) {
		t.Fatalf("settings list = %d: %s", rec.Code, rec.Body.String())
	}

	// Values are writable through PATCH, which leaves metadata alone.
	if rec = srv.do(t, "PATCH", "/api/v1/settings/note", map[string]any{"value": "hello"}); rec.Code != 200 {
		t.Fatalf("value patch = %d: %s", rec.Code, rec.Body.String())
	}
	if store.settings["note"] != "hello" || !store.settingsDisplay["note"] {
		t.Fatalf("patch = %q / %v", store.settings["note"], store.settingsDisplay["note"])
	}

	// Opting out keeps the row, its value and its description.
	rec = srv.do(t, "PATCH", "/api/v1/settings/note", map[string]any{"display": false})
	if rec.Code != 200 {
		t.Fatalf("opt-out patch = %d: %s", rec.Code, rec.Body.String())
	}
	if store.settingsDisplay["note"] {
		t.Fatal("display flag not cleared")
	}
	if store.settings["note"] != "hello" || store.settingsDescriptions["note"] != "A note" {
		t.Fatalf("opt-out should keep the row: %q / %q", store.settings["note"], store.settingsDescriptions["note"])
	}

	// Nothing to update is a validation error.
	if rec := srv.do(t, "PATCH", "/api/v1/settings/note", map[string]any{}); rec.Code != 400 {
		t.Fatalf("empty patch = %d, want 400", rec.Code)
	}

	// Delete removes the row outright - hiding alone is PATCH display:false.
	if rec := srv.do(t, "DELETE", "/api/v1/settings/note", nil); rec.Code != 200 {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := store.settings["note"]; ok {
		t.Fatal("setting row not deleted")
	}
	rec = srv.do(t, "GET", "/api/v1/settings", nil)
	if strings.Contains(rec.Body.String(), `"note"`) {
		t.Fatal("deleted key still listed in settings")
	}
	if rec := srv.do(t, "DELETE", "/api/v1/settings/db_version", nil); rec.Code != 400 {
		t.Fatalf("db_version delete = %d, want 400", rec.Code)
	}
}

// A date-only string must land on UTC midnight of the calendar date: the
// DSN and MySQL session run UTC, so an app-zone parse would store BST
// dates at 23:00 the previous day.
func TestParseDateStoresUTCMidnight(t *testing.T) {
	london := time.FixedZone("Europe/London", 3600)
	api := &API{
		Loc: london,
		Now: func() time.Time { return time.Date(2026, 9, 1, 9, 0, 0, 0, london) },
	}

	at, err := api.parseDate("2026-09-01")
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date-only = %v, want 2026-09-01T00:00:00Z", at)
	}
	if err := api.rejectFuture(at); err != nil {
		t.Fatalf("today rejected: %v", err)
	}

	tomorrow, err := api.parseDate("2026-09-02")
	if err != nil {
		t.Fatal(err)
	}
	if err := api.rejectFuture(tomorrow); err == nil {
		t.Fatal("tomorrow accepted")
	}

	inst, err := api.parseDate("2026-09-01T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if !inst.Equal(time.Date(2026, 9, 1, 11, 0, 0, 0, london)) {
		t.Fatalf("RFC3339 = %v, want the same instant in the app zone", inst)
	}
	if err := api.rejectFuture(inst); err != nil {
		t.Fatalf("same-day instant rejected: %v", err)
	}
}

// The stamp must take the clock from the UTC day, not the local wall clock:
// at 23:30 UTC (00:30 next day in London) a local-clock stamp would pair the
// picked date with London's 00:30 and the UTC write shifts it back a day.
func TestStampNowKeepsDatePart(t *testing.T) {
	london := time.FixedZone("Europe/London", 3600)
	api := &API{
		Loc: london,
		Now: func() time.Time { return time.Date(2026, 9, 1, 23, 30, 5, 0, time.UTC) }, // 00:30 London next day
	}

	at, err := api.parseDate("2026-09-01")
	if err != nil {
		t.Fatal(err)
	}
	stamped := api.stampNow(at)
	want := time.Date(2026, 9, 1, 23, 30, 5, 0, time.UTC)
	if !stamped.Equal(want) {
		t.Fatalf("stamped = %v, want %v (picked date, UTC clock)", stamped, want)
	}
}
