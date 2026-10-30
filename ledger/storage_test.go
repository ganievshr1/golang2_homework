package ledger

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func mockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	oldDB, oldCache := db, cache
	db, cache = conn, nil

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}

		conn.Close()
		db, cache = oldDB, oldCache
	})

	return mock
}

func expectLock(m sqlmock.Sqlmock, category string) {
	m.ExpectBegin()

	m.ExpectExec("SELECT pg_advisory_xact_lock").
		WithArgs(category).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestCreateTransaction(t *testing.T) {
	names := []string{
		"within_limit",
		"exact_limit",
		"exceeded",
		"no_budget",
		"insert_error",
		"commit_error",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			m := mockDB(t)

			tx := Transaction{
				Amount:      100,
				Category:    "еда",
				Description: "ланч",
				Date:        "2026-10-30",
			}

			expectLock(m, tx.Category)

			budget := m.ExpectQuery(
				"SELECT .*limit_amount",
			).WithArgs(tx.Category)

			if name == "no_budget" {
				budget.WillReturnError(sql.ErrNoRows)
			} else {
				budget.WillReturnRows(
					sqlmock.NewRows([]string{"limit"}).
						AddRow(100),
				)

				m.ExpectQuery("SELECT COALESCE").
					WithArgs(
						tx.Category,
						tx.Amount,
						int64(100),
					).
					WillReturnRows(
						sqlmock.NewRows([]string{"exceeded"}).
							AddRow(name == "exceeded"),
					)
			}

			if name == "exceeded" {
				m.ExpectRollback()
			} else {
				insert := m.ExpectQuery(
					"INSERT INTO expenses",
				).WithArgs(
					tx.Amount,
					tx.Category,
					tx.Description,
					tx.Date,
				)

				if name == "insert_error" {
					insert.WillReturnError(
						errors.New("insert failed"),
					)
					m.ExpectRollback()
				} else {
					insert.WillReturnRows(
						sqlmock.NewRows([]string{"id"}).
							AddRow(42),
					)

					if name == "commit_error" {
						m.ExpectCommit().WillReturnError(
							errors.New("commit failed"),
						)
					} else {
						m.ExpectCommit()
					}
				}
			}

			saved, err := CreateTransaction(tx)

			if name == "exceeded" {
				if !errors.Is(err, ErrBudgetExceeded) {
					t.Fatalf("error = %v", err)
				}
			} else if name == "insert_error" ||
				name == "commit_error" {
				if err == nil {
					t.Fatal("storage error ignored")
				}
			} else {
				tx.ID = 42

				if err != nil || saved != tx {
					t.Fatalf(
						"saved=%+v err=%v",
						saved,
						err,
					)
				}
			}
		})
	}
}

func TestInvalidWritesNeverReachDB(t *testing.T) {
	mockDB(t)

	if AddTransaction(Transaction{Amount: -1}) == nil {
		t.Fatal("invalid transaction accepted")
	}

	if SetBudget(Budget{
		Category: "еда",
		Limit:    0,
	}) == nil {
		t.Fatal("invalid budget accepted")
	}

	inputs := []string{
		"",
		"null",
		"{}",
		"[",
		"[null]",
		`[{"category":"еда","limit":0}]`,
		`[{"category":"еда","limit":100},{"category":" ","limit":100}]`,
		`[{"category":"еда","limit":100,"extra":1}]`,
		`[] {}`,
		`[] broken`,
	}

	for _, input := range inputs {
		if LoadBudgets(strings.NewReader(input)) == nil {
			t.Fatalf("accepted %q", input)
		}
	}

	if LoadBudgets(nil) == nil {
		t.Fatal("nil reader accepted")
	}

	if err := LoadBudgets(
		strings.NewReader("[]"),
	); err != nil {
		t.Fatal(err)
	}
}

func TestSetBudgetAndLoadRollback(t *testing.T) {
	m := mockDB(t)

	expectLock(m, "еда")

	m.ExpectExec("INSERT INTO budgets.*ON CONFLICT").
		WithArgs("еда", 501).
		WillReturnResult(sqlmock.NewResult(1, 1))

	m.ExpectCommit()

	if err := SetBudget(Budget{
		Category: "еда",
		Limit:    501,
	}); err != nil {
		t.Fatal(err)
	}

	expectLock(m, "еда")

	m.ExpectExec("INSERT INTO budgets").
		WithArgs("еда", 100).
		WillReturnResult(sqlmock.NewResult(1, 1))

	m.ExpectExec("INSERT INTO budgets").
		WithArgs("еда", 200).
		WillReturnError(errors.New("write failure"))

	m.ExpectRollback()

	input := `[{"category":"еда","limit":100},{"category":"еда","limit":200}]`

	if LoadBudgets(strings.NewReader(input)) == nil {
		t.Fatal("batch error ignored")
	}
}

func TestLists(t *testing.T) {
	m := mockDB(t)

	m.ExpectQuery(
		"SELECT category, .* FROM budgets ORDER BY category",
	).WillReturnRows(
		sqlmock.NewRows([]string{"category", "limit"}).
			AddRow("еда", 501),
	)

	b, err := ListBudgets()
	if err != nil || len(b) != 1 || b[0].Limit != 501 {
		t.Fatalf("budgets=%+v err=%v", b, err)
	}

	columns := []string{
		"id",
		"amount",
		"category",
		"description",
		"date",
	}

	m.ExpectQuery(
		"SELECT id, .* FROM expenses ORDER BY date DESC, id DESC",
	).WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow(7, 501, "еда", "", "2026-10-30"),
	)

	tx, err := ListTransactions()
	if err != nil ||
		len(tx) != 1 ||
		tx[0].ID != 7 ||
		tx[0].Amount != 501 {
		t.Fatalf("transactions=%+v err=%v", tx, err)
	}

	m.ExpectQuery("SELECT category").WillReturnRows(
		sqlmock.NewRows([]string{"category", "limit"}),
	)

	b, err = ListBudgets()
	if err != nil || b == nil || len(b) != 0 {
		t.Fatal("empty budgets must be []")
	}

	m.ExpectQuery("SELECT id").WillReturnRows(
		sqlmock.NewRows(columns),
	)

	tx, err = ListTransactions()
	if err != nil || tx == nil || len(tx) != 0 {
		t.Fatal("empty transactions must be []")
	}

	m.ExpectQuery("SELECT category").
		WillReturnError(errors.New("DB offline"))

	if _, err := ListBudgets(); err == nil {
		t.Fatal("query error ignored")
	}

	m.ExpectQuery("SELECT id").WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow(1, 1, "еда", "", "2026-10-30").
			RowError(0, errors.New("stream error")),
	)

	if _, err := ListTransactions(); err == nil {
		t.Fatal("rows error ignored")
	}
}

func TestReportCache(t *testing.T) {
	m := mockDB(t)
	server := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr:       server.Addr(),
		MaxRetries: -1,
	})
	cache = client

	t.Cleanup(func() {
		client.Close()
	})

	const from, to = "2026-10-01", "2026-10-31"

	key := "report:summary:" + from + ":" + to

	expect := func(total int64) {
		m.ExpectQuery("SELECT category, .*SUM").
			WithArgs(from, to).
			WillReturnRows(
				sqlmock.NewRows([]string{"category", "amount"}).
					AddRow("еда", total),
			)
	}

	expect(450)

	first, err := GetReportSummary(
		context.Background(),
		from,
		to,
	)
	if err != nil || first.Total != 450 {
		t.Fatalf("first=%+v err=%v", first, err)
	}

	if server.TTL(key) != 30*time.Second {
		t.Fatalf("TTL=%s", server.TTL(key))
	}

	// Новый SQL-запрос не ожидается: результат должен прийти из кеша.
	second, err := GetReportSummary(
		context.Background(),
		from,
		to,
	)
	if err != nil || second.Total != 450 {
		t.Fatal("cache hit failed", err)
	}

	server.FastForward(31 * time.Second)
	expect(500)

	third, err := GetReportSummary(
		context.Background(),
		from,
		to,
	)
	if err != nil || third.Total != 500 {
		t.Fatal("cache expiry failed", err)
	}

	server.Set(key, "broken JSON")
	expect(600)

	result, err := GetReportSummary(
		context.Background(),
		from,
		to,
	)
	if err != nil || result.Total != 600 {
		t.Fatal("corrupt cache fallback failed", err)
	}

	server.Close()
	expect(700)

	result, err = GetReportSummary(
		context.Background(),
		from,
		to,
	)
	if err != nil || result.Total != 700 {
		t.Fatal("Redis failure should fall back to DB", err)
	}

	if _, err := GetReportSummary(
		context.Background(),
		to,
		from,
	); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatal(err)
	}
}

func TestDatabaseConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://preferred/db")

	if databaseDSN() != "postgres://preferred/db" {
		t.Fatal("DATABASE_URL priority")
	}

	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "user")
	t.Setenv("DB_PASS", "p@ss:/?#")
	t.Setenv("DB_NAME", "cashapp")

	u, err := url.Parse(databaseDSN())
	if err != nil {
		t.Fatal(err)
	}

	password, _ := u.User.Password()

	if password != "p@ss:/?#" ||
		u.Host != "localhost:5432" ||
		u.Query().Get("sslmode") != "disable" {
		t.Fatal("invalid DSN")
	}

	t.Setenv("REDIS_DB", "wrong")

	if err := Connect(
		context.Background(),
	); err == nil || !strings.Contains(err.Error(), "REDIS_DB") {
		t.Fatal(err)
	}
}
