package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"conspectus/internal/domain"
)

// testServerDSN mirrors the migrations tests: the standard
// CONSPECTUS_MYSQL_* variables when set, else the dev stack's database.
func testServerDSN(t *testing.T) string {
	t.Helper()
	host := os.Getenv("CONSPECTUS_MYSQL_HOST")
	if host == "" {
		if os.Getenv("CONSPECTUS_TEST_MYSQL") == "" {
			t.Skip("MySQL tests need CONSPECTUS_TEST_MYSQL=1 (or CONSPECTUS_MYSQL_HOST set) and a reachable database")
		}
		return "root:devroot@tcp(db:3306)/?parseTime=true&loc=UTC"
	}
	if !strings.Contains(host, ":") {
		host += ":3306"
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/?parseTime=true&loc=UTC",
		os.Getenv("CONSPECTUS_MYSQL_USERNAME"), os.Getenv("CONSPECTUS_MYSQL_PASSWORD"), host)
}

func openTestStore(t *testing.T) (domain.Repos, *sql.DB) {
	t.Helper()
	dsn := testServerDSN(t)
	raw, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Ping(); err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	name := fmt.Sprintf("conspectus_store_%d", rand.Int63())
	if _, err := raw.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() {
		_, _ = raw.Exec("DROP DATABASE IF EXISTS " + name)
		raw.Close()
	})

	loc, _ := time.LoadLocation("UTC")
	repos, err := openDSN(dsnToDB(dsn, name), loc)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { repos.Close() })

	if _, err := raw.Exec("USE " + name); err != nil {
		t.Fatalf("use db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE asset_classes (id INT AUTO_INCREMENT PRIMARY KEY, description VARCHAR(20))`,
		`CREATE TABLE asset_list (id INT AUTO_INCREMENT PRIMARY KEY, asset_class INT NOT NULL, description VARCHAR(40) NOT NULL, closed TINYINT(1) NOT NULL DEFAULT 0)`,
		`CREATE TABLE asset_log (id INT AUTO_INCREMENT PRIMARY KEY, asset_id INT NOT NULL, epoch TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, deposit_value DECIMAL(13,2) NOT NULL, asset_value DECIMAL(13,2) NOT NULL)`,
		`CREATE TABLE payments (id INT AUTO_INCREMENT PRIMARY KEY, asset_id INT NOT NULL, epoch TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, amount DECIMAL(13,2) NOT NULL)`,
		`CREATE TABLE settings (setting VARCHAR(20) PRIMARY KEY, value VARCHAR(2048) NOT NULL, description VARCHAR(120) NOT NULL DEFAULT '', display TINYINT(1) NOT NULL DEFAULT 0)`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("schema %q: %v", stmt, err)
		}
	}
	return repos, raw
}

func dsnToDB(dsn, db string) string {
	i := strings.LastIndexByte(dsn, '/')
	if q := strings.IndexByte(dsn[i:], '?'); q >= 0 {
		return dsn[:i+1] + db + dsn[i:][q:]
	}
	return dsn[:i+1] + db
}

func TestAssetLifecycle(t *testing.T) {
	repos, _ := openTestStore(t)
	ctx := context.Background()

	class, err := repos.Classes().Create(ctx, domain.Class{Description: "Savings"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Classes().Create(ctx, domain.Class{Description: "savings"}); err == nil {
		t.Fatal("duplicate class allowed")
	}

	asset, err := repos.Assets().Create(ctx, domain.Asset{ClassID: class.ID, Description: "Chase Saver"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Assets().Create(ctx, domain.Asset{ClassID: class.ID, Description: "CHASE SAVER"}); err == nil {
		t.Fatal("duplicate asset allowed")
	}

	asset.Description = "Chase Saver 2"
	if _, err := repos.Assets().Update(ctx, asset); err != nil {
		t.Fatal(err)
	}
	if err := repos.Assets().SetClosed(ctx, asset.ID, true); err != nil {
		t.Fatal(err)
	}
	got, _ := repos.Assets().Get(ctx, asset.ID)
	if !got.Closed || got.Description != "Chase Saver 2" {
		t.Fatalf("asset = %+v", got)
	}

	open := false
	closed := true
	list, total, err := repos.Assets().List(ctx, domain.AssetFilter{Closed: &open})
	if err != nil || total != 0 {
		t.Fatalf("open filter = %v %d %v", list, total, err)
	}
	list, total, _ = repos.Assets().List(ctx, domain.AssetFilter{Closed: &closed, Query: "chase"})
	if total != 1 {
		t.Fatalf("closed+query filter total = %d", total)
	}
	if len(list) != 1 || list[0].ID != asset.ID {
		t.Fatalf("closed+query list = %+v", list)
	}

	if _, err := repos.Logs().Create(ctx, domain.LogEntry{
		AssetID: asset.ID, At: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Deposit: money("100"), Value: money("100"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repos.Assets().Delete(ctx, asset.ID, false); err == nil {
		t.Fatal("delete allowed with rows")
	}

	if err := repos.Classes().Delete(ctx, class.ID); err == nil {
		t.Fatal("class delete allowed with assets")
	}
}

func TestLogPaymentsAndSeries(t *testing.T) {
	repos, raw := openTestStore(t)
	ctx := context.Background()
	class, _ := repos.Classes().Create(ctx, domain.Class{Description: "Savings"})
	a1, _ := repos.Assets().Create(ctx, domain.Asset{ClassID: class.ID, Description: "Alpha"})
	a2, _ := repos.Assets().Create(ctx, domain.Asset{ClassID: class.ID, Description: "Beta"})

	jan := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	if _, err := repos.Logs().Create(ctx, domain.LogEntry{AssetID: a1.ID, At: jan, Deposit: money("1000"), Value: money("1010")}); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Logs().Create(ctx, domain.LogEntry{AssetID: a1.ID, At: jan.AddDate(0, 0, 3), Deposit: money("1000"), Value: money("1011")}); err == nil {
		t.Fatal("duplicate month allowed")
	}
	if _, err := raw.Exec("INSERT INTO asset_log (asset_id, epoch, deposit_value, asset_value) VALUES (?, '2026-03-02 09:00:00', '1200.00', '1212.00')", a1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO asset_log (asset_id, epoch, deposit_value, asset_value) VALUES (?, '2026-03-28 09:00:00', '1200.00', '1215.00')", a1.ID); err != nil {
		t.Fatal(err)
	}

	repos.Logs().Create(ctx, domain.LogEntry{AssetID: a2.ID, At: feb, Deposit: money("500"), Value: money("500")})
	repos.Payments().Create(ctx, domain.Payment{AssetID: a1.ID, At: feb, Amount: money("8")})

	rows, err := repos.Logs().SeriesRows(ctx, domain.SeriesQuery{})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]domain.SeriesRow{}
	for _, r := range rows {
		byKey[fmt.Sprintf("%d:%s", r.AssetID, r.Month.Key())] = r
	}
	mar := byKey[fmt.Sprintf("%d:2026-03", a1.ID)]
	if mar.Value.String() != "1215.00" || mar.Entries != 2 {
		t.Fatalf("March dedup = %+v", mar)
	}
	febA1 := byKey[fmt.Sprintf("%d:2026-02", a1.ID)]
	if febA1.Payments.String() != "8.00" {
		t.Fatalf("February payments = %+v", febA1)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d (%+v)", len(rows), rows)
	}

	from := domain.Month{Year: 2026, Month: time.March}
	to := domain.Month{Year: 2026, Month: time.March}
	rows, _ = repos.Logs().SeriesRows(ctx, domain.SeriesQuery{From: &from, To: &to})
	if len(rows) != 1 {
		t.Fatalf("bounded rows = %d", len(rows))
	}

	logs, total, err := repos.Logs().List(ctx, domain.LogFilter{Query: "alpha"})
	if err != nil || total != 3 {
		t.Fatalf("query filter = %d %v", total, err)
	}
	if len(logs) != 3 {
		t.Fatalf("logs = %d", len(logs))
	}

	logsDeleted, paysDeleted, err := repos.Assets().Delete(ctx, a1.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if logsDeleted != 3 || paysDeleted != 1 {
		t.Fatalf("cascade counts = %d/%d", logsDeleted, paysDeleted)
	}
}

func TestSettingsRepo(t *testing.T) {
	repos, _ := openTestStore(t)
	ctx := context.Background()
	if _, err := repos.Settings().Get(ctx, "missing"); err == nil {
		t.Fatal("missing key should 404")
	}
	if err := repos.Settings().Set(ctx, "ui_note", "one"); err != nil {
		t.Fatal(err)
	}
	if err := repos.Settings().Set(ctx, "ui_note", "two"); err != nil {
		t.Fatal(err)
	}
	v, err := repos.Settings().Get(ctx, "ui_note")
	if err != nil || v != "two" {
		t.Fatalf("get = %q %v", v, err)
	}
	all, _ := repos.Settings().All(ctx)
	if all["ui_note"] != "two" {
		t.Fatalf("all = %v", all)
	}
	if err := repos.Settings().Set(ctx, "this_key_is_way_too_long", "x"); err == nil {
		t.Fatal("long key allowed")
	}

	// Opting in on an absent key creates the row; opting out keeps it.
	if err := repos.Settings().SetDescription(ctx, "ui_note", "A note"); err != nil {
		t.Fatal(err)
	}
	if err := repos.Settings().SetDisplay(ctx, "ui_note", true); err != nil {
		t.Fatal(err)
	}
	shown, err := repos.Settings().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range shown {
		if s.Key == "ui_note" {
			found = true
			if !s.Display || s.Value != "two" || s.Description != "A note" {
				t.Fatalf("listed row = %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("opted-in key missing from List")
	}
	if err := repos.Settings().SetDisplay(ctx, "ui_note", false); err != nil {
		t.Fatal(err)
	}
	shown, _ = repos.Settings().List(ctx)
	for _, s := range shown {
		if s.Key == "ui_note" && s.Display {
			t.Fatal("opted-out key still flagged displayed")
		}
	}
	if v, err := repos.Settings().Get(ctx, "ui_note"); err != nil || v != "two" {
		t.Fatalf("opt-out kept value: %q %v", v, err)
	}
	// Opting out never creates rows.
	if err := repos.Settings().SetDisplay(ctx, "never_seen", false); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Settings().Get(ctx, "never_seen"); err == nil {
		t.Fatal("opt-out created a row")
	}

	// Create inserts a fresh row with metadata; a duplicate is a conflict.
	if err := repos.Settings().Create(ctx, "new_key", "v1", "A label", true); err != nil {
		t.Fatal(err)
	}
	if v, err := repos.Settings().Get(ctx, "new_key"); err != nil || v != "v1" {
		t.Fatalf("created value = %q %v", v, err)
	}
	shown, _ = repos.Settings().List(ctx)
	found = false
	for _, s := range shown {
		if s.Key == "new_key" {
			found = true
			if !s.Display || s.Value != "v1" || s.Description != "A label" {
				t.Fatalf("created row = %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("created key missing from List")
	}
	if err := repos.Settings().Create(ctx, "new_key", "v2", "", false); err == nil {
		t.Fatal("duplicate create allowed")
	}
}

func TestTransactionRollback(t *testing.T) {
	repos, _ := openTestStore(t)
	ctx := context.Background()
	err := repos.Tx(ctx, func(tx domain.Repos) error {
		if _, err := tx.Classes().Create(ctx, domain.Class{Description: "TxClass"}); err != nil {
			return err
		}
		return fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	classes, _ := repos.Classes().List(ctx)
	for _, c := range classes {
		if c.Description == "TxClass" {
			t.Fatal("rollback failed: class persisted")
		}
	}
}

func money(major string) domain.Money {
	v, err := domain.ParseMoney(major)
	if err != nil {
		panic(err)
	}
	return v
}
