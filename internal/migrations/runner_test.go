package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

// resolveTestDSN returns a server-level DSN: the standard CONSPECTUS_MYSQL_*
// variables when set, else the dev stack's database service.
func resolveTestDSN(t *testing.T) string {
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

func testDSN(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := resolveTestDSN(t)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}
	name := fmt.Sprintf("conspectus_test_%d", rand.Int63())
	if _, err := db.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP DATABASE IF EXISTS " + name)
		db.Close()
	})
	dbc, err := sql.Open("mysql", dsnMixin(dsn, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbc.Close() })
	return dbc, name
}

func dsnMixin(dsn, db string) string {
	i := strings.LastIndexByte(dsn, '/')
	if j := strings.IndexByte(dsn[i+1:], '?'); j >= 0 {
		return dsn[:i+1] + db + dsn[i+1:][j:]
	}
	return dsn[:i+1] + db
}

func testSet() []Migration {
	return []Migration{
		{Version: 1, Name: "0001_test.sql", Contents: `
			CREATE TABLE IF NOT EXISTS settings (setting VARCHAR(20) PRIMARY KEY, value VARCHAR(2048) NOT NULL);
			CREATE TABLE IF NOT EXISTS alpha (id INT PRIMARY KEY, note VARCHAR(10));
			CREATE TABLE IF NOT EXISTS beta (id INT PRIMARY KEY);
			INSERT INTO alpha (id, note) VALUES (1, 'one');
		`},
		{Version: 2, Name: "0002_test.sql", Contents: `
			CREATE INDEX idx_beta ON beta (id);
			-- a comment; also exercising the splitter
			INSERT INTO beta (id) VALUES (7);
		`},
	}
}

func TestFreshRunAppliesAll(t *testing.T) {
	db, _ := testDSN(t)
	r := &Runner{DB: db, Set: testSet()}
	res, err := r.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 2 || res.To != 2 {
		t.Fatalf("res = %+v", res)
	}
	res, err = r.Run(context.Background())
	if err != nil || len(res.Applied) != 0 {
		t.Fatalf("re-run = %+v err=%v", res, err)
	}
	var note string
	if err := db.QueryRow("SELECT note FROM alpha WHERE id = 1").Scan(&note); err != nil || note != "one" {
		t.Fatalf("data check: %v %q", err, note)
	}
}

func seedV1(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE asset_classes (id INT AUTO_INCREMENT PRIMARY KEY, description VARCHAR(20))`,
		`CREATE TABLE asset_list (id INT AUTO_INCREMENT PRIMARY KEY, asset_class INT NOT NULL, description VARCHAR(40) NOT NULL, closed TINYINT(1) NOT NULL DEFAULT 0)`,
		`CREATE TABLE asset_log (id INT AUTO_INCREMENT PRIMARY KEY, asset_id INT NOT NULL, epoch TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, deposit_value DECIMAL(13,2) NOT NULL, asset_value DECIMAL(13,2) NOT NULL)`,
		`CREATE TABLE settings (setting VARCHAR(20) PRIMARY KEY, value VARCHAR(20) NOT NULL)`,
		`INSERT INTO settings (setting, value) VALUES ('db_version', '3')`,
		`INSERT INTO asset_classes (description) VALUES ('Savings')`,
		`INSERT INTO asset_list (asset_class, description) VALUES (1, 'Old Asset')`,
		`INSERT INTO asset_log (asset_id, epoch, deposit_value, asset_value) VALUES (1, '2020-01-31', '100.00', '110.00')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
}

func realSet() []Migration {
	return []Migration{
		{Version: 1, Name: "0001_base.sql", Contents: "-- subsumed by the v1 history"},
		{Version: 2, Name: "0002_asset_closed.sql", Contents: "-- subsumed"},
		{Version: 3, Name: "0003_utf8mb4.sql", Contents: "-- subsumed"},
		{Version: 4, Name: "0004_payments.sql", Contents: `CREATE TABLE payments (id INT AUTO_INCREMENT PRIMARY KEY, asset_id INT NOT NULL, epoch TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, amount DECIMAL(13,2) NOT NULL)`},
		{Version: 5, Name: "0005_settings_value.sql", Contents: `ALTER TABLE settings MODIFY value VARCHAR(2048) NOT NULL`},
		{Version: 6, Name: "0006_indexes.sql", Contents: "CREATE INDEX idx_log ON asset_log (asset_id, epoch);\nALTER TABLE settings ADD COLUMN description VARCHAR(120) NOT NULL DEFAULT '', ADD COLUMN display TINYINT(1) NOT NULL DEFAULT 0"},
	}
}

func TestUpgradeV1DatabaseInPlace(t *testing.T) {
	db, _ := testDSN(t)
	seedV1(t, db)
	r := &Runner{DB: db, Set: realSet()}
	res, err := r.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.From != 3 || res.To != 6 || len(res.Applied) != 3 {
		t.Fatalf("res = %+v", res)
	}
	var dep, val string
	if err := db.QueryRow("SELECT deposit_value, asset_value FROM asset_log WHERE asset_id = 1").Scan(&dep, &val); err != nil {
		t.Fatal(err)
	}
	if dep != "100.00" || val != "110.00" {
		t.Fatalf("data changed: %s/%s", dep, val)
	}
	if _, err := db.Exec(`INSERT INTO settings (setting, value) VALUES ('k', REPEAT('x', 100))`); err != nil {
		t.Fatalf("settings still narrow: %v", err)
	}
	res, err = r.Run(context.Background())
	if err != nil || len(res.Applied) != 0 {
		t.Fatalf("re-run = %+v err=%v", res, err)
	}
}

func TestRunRefusesNewerSchema(t *testing.T) {
	db, _ := testDSN(t)
	if _, err := db.Exec(`CREATE TABLE settings (setting VARCHAR(20) PRIMARY KEY, value VARCHAR(2048) NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO settings (setting, value) VALUES ('db_version', '9')`); err != nil {
		t.Fatal(err)
	}
	r := &Runner{DB: db, Set: testSet()}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("expected downgrade refusal")
	}
}

func TestGaplessEnforcement(t *testing.T) {
	db, _ := testDSN(t)
	gappy := []Migration{
		{Version: 1, Name: "0001_a.sql", Contents: "CREATE TABLE g (id INT PRIMARY KEY)"},
		{Version: 3, Name: "0003_c.sql", Contents: "SELECT 1"},
	}
	r := &Runner{DB: db, Set: gappy}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("expected gapless failure")
	}
}

func TestResumeAfterFailure(t *testing.T) {
	db, _ := testDSN(t)
	set := []Migration{
		{Version: 1, Name: "0001_ok.sql", Contents: "CREATE TABLE IF NOT EXISTS settings (setting VARCHAR(20) PRIMARY KEY, value VARCHAR(2048) NOT NULL);\nCREATE TABLE r1 (id INT PRIMARY KEY)"},
		{Version: 2, Name: "0002_fail.sql", Contents: "CREATE TABLE r2 (id INT PRIMARY KEY);\nTHIS IS NOT SQL;"},
		{Version: 3, Name: "0003_after.sql", Contents: "CREATE TABLE r3 (id INT PRIMARY KEY)"},
	}
	r := &Runner{DB: db, Set: set}
	if _, err := r.Run(context.Background()); err == nil {
		t.Fatal("expected failure on bad statement")
	}
	var v string
	db.QueryRow("SELECT value FROM settings WHERE setting = 'db_version'").Scan(&v)
	if v != "1" {
		t.Fatalf("version after failure = %q", v)
	}
	st, ok := r.readState(context.Background())
	if !ok || st.File != "0002_fail.sql" {
		t.Fatalf("state = %+v ok=%v", st, ok)
	}
	set[1].Contents = "CREATE TABLE r2 (id INT PRIMARY KEY, ok INT)"
	if _, err := r.Repair(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := r.Run(context.Background())
	if err != nil || res.To != 3 {
		t.Fatalf("post-repair run = %+v err=%v", res, err)
	}
}

func TestConcurrentRunnersSerialise(t *testing.T) {
	db, _ := testDSN(t)
	r1 := &Runner{DB: db, Set: testSet()}
	r2 := &Runner{DB: db, Set: testSet()}
	errCh := make(chan error, 2)
	go func() { _, err := r1.Run(context.Background()); errCh <- err }()
	time.Sleep(5 * time.Millisecond)
	go func() { _, err := r2.Run(context.Background()); errCh <- err }()
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent run: %v", err)
		}
	}
	var n int
	db.QueryRow("SELECT COUNT(*) FROM alpha WHERE id = 1").Scan(&n)
	if n != 1 {
		t.Fatalf("alpha rows = %d", n)
	}
}
