//go:build integration

package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Тест создаёт отдельную схему и удаляет только её.
func TestPostgresRedisIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal(
			"задайте TEST_DATABASE_URL для интеграционного теста",
		)
	}

	ctx := context.Background()

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	schema := fmt.Sprintf(
		"ledger_test_%d",
		time.Now().UnixNano(),
	)

	if _, err := admin.Exec(
		"CREATE SCHEMA " + schema,
	); err != nil {
		t.Fatal(err)
	}

	defer func() {
		Close()

		if _, err := admin.Exec(
			"DROP SCHEMA " + schema + " CASCADE",
		); err != nil {
			t.Error(err)
		}
	}()

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}

	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	t.Setenv("DATABASE_URL", u.String())
	t.Setenv("REDIS_DB", envOr("TEST_REDIS_DB", "15"))

	if err := Connect(ctx); err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob("migrations/*.sql")
	if err != nil || len(files) != 2 {
		t.Fatalf("migrations=%v err=%v", files, err)
	}

	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		up := strings.Split(
			string(body),
			"-- +goose Down",
		)[0]

		if _, err := db.Exec(up); err != nil {
			t.Fatal(err)
		}
	}

	if db.Stats().MaxOpenConnections != 10 {
		t.Fatal("pool limit is not 10")
	}

	if list, err := ListTransactions(); err != nil ||
		list == nil ||
		len(list) != 0 {
		t.Fatalf(
			"empty transactions=%v err=%v",
			list,
			err,
		)
	}

	if list, err := ListBudgets(); err != nil ||
		list == nil ||
		len(list) != 0 {
		t.Fatalf(
			"empty budgets=%v err=%v",
			list,
			err,
		)
	}

	for _, limit := range []int{1500, 1000} {
		if err := SetBudget(Budget{
			Category: "еда",
			Limit:    limit,
		}); err != nil {
			t.Fatal(err)
		}
	}

	b, err := ListBudgets()
	if err != nil || len(b) != 1 || b[0].Limit != 1000 {
		t.Fatalf("upsert=%v err=%v", b, err)
	}

	first, err := CreateTransaction(Transaction{
		Amount:   450,
		Category: "еда",
		Date:     "2099-10-01",
	})
	if err != nil || first.ID <= 0 {
		t.Fatalf("create=%+v err=%v", first, err)
	}

	if err := AddTransaction(Transaction{
		Amount:   600,
		Category: "еда",
		Date:     "2099-10-01",
	}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatal("budget not enforced", err)
	}

	second, err := CreateTransaction(Transaction{
		Amount:   550,
		Category: "еда",
		Date:     "2099-10-01",
	})
	if err != nil || second.ID <= first.ID {
		t.Fatal("exact limit", err)
	}

	third, err := CreateTransaction(Transaction{
		Amount:   500,
		Category: "досуг",
		Date:     "2099-10-02",
	})
	if err != nil {
		t.Fatal("no budget", err)
	}

	if AddTransaction(Transaction{
		Amount:   0,
		Category: "еда",
		Date:     "2099-10-01",
	}) == nil {
		t.Fatal("invalid accepted")
	}

	list, err := ListTransactions()
	if err != nil ||
		len(list) != 3 ||
		list[0].ID != third.ID ||
		list[1].ID != second.ID ||
		list[2].ID != first.ID {
		t.Fatalf(
			"order/persistence=%+v err=%v",
			list,
			err,
		)
	}

	var rubles string

	err = db.QueryRow(
		"SELECT amount::text FROM expenses WHERE id=$1",
		first.ID,
	).Scan(&rubles)

	if err != nil || rubles != "4.50" {
		t.Fatalf("money=%s err=%v", rubles, err)
	}

	const from, to = "2099-10-01", "2099-10-31"

	key := "report:summary:" + from + ":" + to

	if err := cache.Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if cache != nil {
			cache.Del(ctx, key)
		}
	}()

	report, err := GetReportSummary(ctx, from, to)
	if err != nil ||
		report.Total != 1500 ||
		len(report.Items) != 2 {
		t.Fatalf("report=%+v err=%v", report, err)
	}

	ttl, err := cache.TTL(ctx, key).Result()
	if err != nil ||
		ttl <= 0 ||
		ttl > 30*time.Second {
		t.Fatalf("TTL=%s err=%v", ttl, err)
	}

	if err := AddTransaction(Transaction{
		Amount:   100,
		Category: "досуг",
		Date:     "2099-10-02",
	}); err != nil {
		t.Fatal(err)
	}

	// До истечения TTL отчёт остаётся прежним.
	hit, err := GetReportSummary(ctx, from, to)
	if err != nil || hit.Total != 1500 {
		t.Fatal("cache hit", err)
	}

	if err := cache.PExpire(
		ctx,
		key,
		time.Millisecond,
	).Err(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)

	fresh, err := GetReportSummary(ctx, from, to)
	if err != nil || fresh.Total != 1600 {
		t.Fatal("cache expiration", err)
	}

	if err := Close(); err != nil {
		t.Fatal(err)
	}

	if err := Connect(ctx); err != nil {
		t.Fatal(err)
	}

	if list, err := ListTransactions(); err != nil ||
		len(list) != 4 {
		t.Fatal("data lost after reconnect", err)
	}

	if err := SetBudget(Budget{
		Category: "parallel",
		Limit:    5,
	}); err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 20)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			results <- AddTransaction(Transaction{
				Amount:   1,
				Category: "parallel",
				Date:     "2099-11-01",
			})
		}()
	}

	wg.Wait()
	close(results)

	successes := 0

	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatal(err)
		}
	}

	if successes != 5 {
		t.Fatalf(
			"successful concurrent writes=%d want=5",
			successes,
		)
	}
}
