package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"example.com/finance-services/ledger"
)

func request(
	t *testing.T,
	h http.Handler,
	method, path, body string,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)
	r.Header.Set("Content-Type", "application/json")

	h.ServeHTTP(w, r)

	if w.Code != status {
		t.Fatalf(
			"%s %s: status=%d want=%d body=%s",
			method,
			path,
			w.Code,
			status,
			w.Body.String(),
		)
	}

	if w.Header().Get("Content-Type") != jsonContentType ||
		!json.Valid(w.Body.Bytes()) {
		t.Fatalf("invalid JSON response: %s", w.Body.String())
	}

	if status >= 400 {
		var result map[string]string

		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil ||
			len(result) != 1 ||
			result["error"] == "" {
			t.Fatalf("invalid error response: %s", w.Body.String())
		}
	}

	return w
}

func transactionJSON(amount int, category string) string {
	data, _ := json.Marshal(CreateTransactionRequest{
		Amount:      amount,
		Category:    category,
		Description: "Ланч",
		Date:        "2026-10-18",
	})

	return string(data)
}

func TestLedgerHTTP(t *testing.T) {
	var logs bytes.Buffer
	h := NewHandler(log.New(&logs, "", 0))
	s := ledgerStore{}

	before := len(s.ListTransactions())
	category := fmt.Sprintf("еда-%d", before)

	budgetBody := fmt.Sprintf(
		`{"category":%q,"limit":1000}`,
		category,
	)

	w := request(
		t, h, "POST", "/api/budgets", budgetBody, 201,
	)

	var budget BudgetResponse
	if err := json.Unmarshal(w.Body.Bytes(), &budget); err != nil ||
		budget.Category != category ||
		budget.Limit != 1000 {
		t.Fatalf("budget response: %s", w.Body.String())
	}

	for i, amount := range []int{450, 550} {
		w = request(
			t, h, "POST", "/api/transactions",
			transactionJSON(amount, category), 201,
		)

		var tx TransactionResponse
		if err := json.Unmarshal(w.Body.Bytes(), &tx); err != nil {
			t.Fatal(err)
		}

		if tx.ID != before+i+1 ||
			tx.Amount != amount ||
			tx.Category != category ||
			tx.Description != "Ланч" ||
			tx.Date != "2026-10-18" {
			t.Fatalf("transaction response: %+v", tx)
		}
	}

	w = request(
		t, h, "POST", "/api/transactions",
		transactionJSON(1, category), 409,
	)

	if strings.TrimSpace(w.Body.String()) !=
		`{"error":"budget exceeded"}` {
		t.Fatal(w.Body.String())
	}

	request(
		t, h, "POST", "/api/transactions",
		transactionJSON(-1, category), 400,
	)

	request(
		t, h, "POST", "/api/budgets",
		fmt.Sprintf(`{"category":%q,"limit":0}`, category),
		400,
	)

	if len(s.ListTransactions()) != before+2 {
		t.Fatal("rejected transaction was saved")
	}

	request(
		t, h, "POST", "/api/budgets",
		fmt.Sprintf(`{"category":%q,"limit":1100}`, category),
		201,
	)

	request(
		t, h, "POST", "/api/transactions",
		transactionJSON(100, category), 201,
	)

	w = request(t, h, "GET", "/api/transactions", "", 200)

	var all []TransactionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil ||
		len(all) != before+3 ||
		all[len(all)-1].ID != before+3 {
		t.Fatalf("wrong saved history: %s", w.Body.String())
	}

	w = request(t, h, "GET", "/api/budgets", "", 200)

	var budgets []BudgetResponse
	if err := json.Unmarshal(w.Body.Bytes(), &budgets); err != nil {
		t.Fatal(err)
	}

	found := 0
	for _, b := range budgets {
		if b.Category == category {
			found++
			if b.Limit != 1100 {
				t.Fatal("budget update failed")
			}
		}
	}

	if found != 1 {
		t.Fatal("budget duplicated or missing")
	}

	for _, fragment := range []string{
		"method=POST",
		"method=GET",
		`path="/api/transactions"`,
		`path="/api/budgets"`,
		"duration=",
	} {
		if !strings.Contains(logs.String(), fragment) {
			t.Fatalf("missing log %q", fragment)
		}
	}
}

// Заглушка воспроизводит внутренние ошибки
// без изменения рабочей реализации Ledger.
type failingStore struct {
	err   error
	calls int
}

func (s *failingStore) AddTransaction(
	ledger.Transaction,
) (ledger.Transaction, error) {
	s.calls++
	return ledger.Transaction{}, s.err
}

func (s *failingStore) SetBudget(ledger.Budget) error {
	s.calls++
	return s.err
}

func (*failingStore) ListTransactions() []ledger.Transaction {
	return nil
}

func (*failingStore) ListBudgets() []ledger.Budget {
	return nil
}

func TestInvalidRequestsAndEmptyLists(t *testing.T) {
	s := &failingStore{
		err: errors.New("must not be called"),
	}
	h := newHandler(s, log.New(io.Discard, "", 0))

	for _, path := range []string{
		"/api/transactions",
		"/api/budgets",
	} {
		w := request(t, h, "GET", path, "", 200)

		if strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal(w.Body.String())
		}

		for _, body := range []string{
			"",
			"null",
			"[]",
			"{",
			"{}",
			`{"unknown":1}`,
			`{} {}`,
			`{} broken`,
			strings.Repeat(" ", maxBodyBytes+1),
		} {
			request(t, h, "POST", path, body, 400)
		}
	}

	for _, body := range []string{
		transactionJSON(0, "еда"),
		transactionJSON(-1, "еда"),
		transactionJSON(1, " "),
		`{"amount":1,"category":"еда","date":"2026-02-30"}`,
		`{"amount":1,"category":"еда"}`,
		`{"amount":"100","category":"еда","date":"2026-10-18"}`,
		`{"amount":1.5,"category":"еда","date":"2026-10-18"}`,
	} {
		request(
			t, h, "POST", "/api/transactions", body, 400,
		)
	}

	for _, body := range []string{
		`{"category":"еда","limit":0}`,
		`{"category":"еда","limit":-1}`,
		`{"category":" ","limit":100}`,
		`{"category":"еда","limit":"100"}`,
	} {
		request(t, h, "POST", "/api/budgets", body, 400)
	}

	if s.calls != 0 {
		t.Fatal("invalid request reached store")
	}
}

func TestInternalErrors(t *testing.T) {
	tests := []struct {
		path   string
		body   string
		err    error
		status int
	}{
		{
			"/api/transactions",
			transactionJSON(100, "еда"),
			errors.New("private storage failure"),
			500,
		},
		{
			"/api/budgets",
			`{"category":"еда","limit":100}`,
			errors.New("private storage failure"),
			500,
		},
		{
			"/api/transactions",
			transactionJSON(100, "еда"),
			fmt.Errorf("wrapped: %w", ledger.ErrBudgetExceeded),
			409,
		},
	}

	for _, tt := range tests {
		var logs bytes.Buffer
		s := &failingStore{err: tt.err}
		h := newHandler(s, log.New(&logs, "", 0))

		w := request(
			t, h, "POST", tt.path, tt.body, tt.status,
		)

		if s.calls != 1 {
			t.Fatal("store not called")
		}

		if tt.status == 500 {
			if strings.TrimSpace(w.Body.String()) !=
				`{"error":"internal server error"}` ||
				!strings.Contains(logs.String(), tt.err.Error()) {
				t.Fatal("internal error was not handled correctly")
			}
		}
	}
}

func TestConcurrentRequests(t *testing.T) {
	s := ledgerStore{}
	before := len(s.ListTransactions())
	category := fmt.Sprintf("concurrent-%d", before)

	if err := s.SetBudget(ledger.Budget{
		Category: category,
		Limit:    12,
	}); err != nil {
		t.Fatal(err)
	}

	handlers := []http.Handler{
		NewHandler(log.New(io.Discard, "", 0)),
		NewHandler(log.New(io.Discard, "", 0)),
	}

	results := make(chan *httptest.ResponseRecorder, 40)
	var wg sync.WaitGroup

	for i := 0; i < 40; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(
				"POST",
				"/api/transactions",
				strings.NewReader(transactionJSON(1, category)),
			)

			handlers[i%2].ServeHTTP(w, r)
			results <- w

			handlers[i%2].ServeHTTP(
				httptest.NewRecorder(),
				httptest.NewRequest("GET", "/api/budgets", nil),
			)

			handlers[i%2].ServeHTTP(
				httptest.NewRecorder(),
				httptest.NewRequest("GET", "/api/transactions", nil),
			)
		}(i)
	}

	wg.Wait()
	close(results)

	ids := make(map[int]bool)

	for w := range results {
		if w.Code == 409 {
			continue
		}

		if w.Code != 201 {
			t.Fatalf(
				"unexpected response: %d %s",
				w.Code,
				w.Body.String(),
			)
		}

		var tx TransactionResponse
		if err := json.Unmarshal(w.Body.Bytes(), &tx); err != nil {
			t.Fatal(err)
		}

		if ids[tx.ID] ||
			tx.ID <= before ||
			tx.Category != category {
			t.Fatalf("wrong ID or category: %+v", tx)
		}

		ids[tx.ID] = true
	}

	if len(ids) != 12 ||
		len(s.ListTransactions()) != before+12 {
		t.Fatal("concurrent budget check failed")
	}
}
