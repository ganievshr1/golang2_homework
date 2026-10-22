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
	"reflect"
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

	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
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

// Тесты с настоящим Ledger выполняются последовательно.
// Сброс выполняется до сценария и после завершения всех его запросов.
func resetLedger(t *testing.T) {
	t.Helper()
	ledger.Reset()
	t.Cleanup(ledger.Reset)
}

func TestBudgetsHTTP(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		update bool
	}{
		{"ok", `{"category":"еда","limit":5000}`, 201, false},
		{"update", `{"category":"еда","limit":5000}`, 201, true},
		{"bad_json", `{"category":`, 400, false},
		{"zero_limit", `{"category":"еда","limit":0}`, 400, false},
		{"negative_limit", `{"category":"еда","limit":-1}`, 400, false},
		{"empty_category", `{"category":"","limit":5000}`, 400, false},
		{"blank_category", `{"category":" ","limit":5000}`, 400, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLedger(t)
			h := NewHandler(log.New(io.Discard, "", 0))

			if tt.update {
				request(t, h, "POST", "/api/budgets",
					`{"category":"еда","limit":1000}`, 201)
			}

			w := request(t, h, "POST", "/api/budgets", tt.body, tt.status)
			want := make([]BudgetResponse, 0)
			if tt.status == 201 {
				expected := BudgetResponse{Category: "еда", Limit: 5000}
				var created BudgetResponse
				if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
					t.Fatal(err)
				}
				if created != expected {
					t.Fatalf("created = %+v, want %+v", created, expected)
				}
				want = append(want, expected)
			}

			w = request(t, h, "GET", "/api/budgets", "", 200)
			var got []BudgetResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("GET budgets = %+v, want %+v", got, want)
			}
		})
	}
}

func TestTransactionsHTTP(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		amount int
		status int
	}{
		{"ok", transactionJSON(1000, "еда"), 1000, 201},
		{"exact_limit", transactionJSON(3500, "еда"), 3500, 201},
		{"exceeded", transactionJSON(3501, "еда"), 0, 409},
		{"bad_json", `{"amount":`, 0, 400},
		{"zero_amount", transactionJSON(0, "еда"), 0, 400},
		{"negative_amount", transactionJSON(-100, "еда"), 0, 400},
		{"empty_category", transactionJSON(100, ""), 0, 400},
		{"bad_date", `{"amount":100,"category":"еда","date":"2026-02-30"}`, 0, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetLedger(t)
			h := NewHandler(log.New(io.Discard, "", 0))

			request(t, h, "POST", "/api/budgets",
				`{"category":"еда","limit":5000}`, 201)
			w := request(t, h, "POST", "/api/transactions",
				transactionJSON(1500, "еда"), 201)

			first := TransactionResponse{
				ID: 1, Amount: 1500, Category: "еда",
				Description: "Ланч", Date: "2026-10-18",
			}
			var created TransactionResponse
			if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			if created != first {
				t.Fatalf("first transaction = %+v, want %+v", created, first)
			}
			want := []TransactionResponse{first}

			w = request(t, h, "POST", "/api/transactions", tt.body, tt.status)
			if tt.status == 201 {
				expected := TransactionResponse{
					ID: 2, Amount: tt.amount, Category: "еда",
					Description: "Ланч", Date: "2026-10-18",
				}
				if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
					t.Fatal(err)
				}
				if created != expected {
					t.Fatalf("created = %+v, want %+v", created, expected)
				}
				want = append(want, expected)
			} else if tt.status == 409 {
				if strings.TrimSpace(w.Body.String()) != `{"error":"budget exceeded"}` {
					t.Fatalf("wrong budget error: %s", w.Body.String())
				}
			}

			w = request(t, h, "GET", "/api/transactions", "", 200)
			var got []TransactionResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("GET transactions = %+v, want %+v", got, want)
			}
		})
	}
}

func TestLogging(t *testing.T) {
	var logs bytes.Buffer
	h := newHandler(&failingStore{}, log.New(&logs, "", 0))
	request(t, h, "GET", "/api/transactions", "", 200)
	request(t, h, "POST", "/api/budgets", "{", 400)

	for _, fragment := range []string{
		"method=GET", "method=POST", "duration=",
		`path="/api/transactions"`, `path="/api/budgets"`,
	} {
		if !strings.Contains(logs.String(), fragment) {
			t.Fatalf("missing log %q: %s", fragment, logs.String())
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
	resetLedger(t)
	s := ledgerStore{}
	const category = "конкурентные запросы"

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
		if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("wrong Content-Type: %q", w.Header().Get("Content-Type"))
		}
		if w.Code == 409 {
			if strings.TrimSpace(w.Body.String()) != `{"error":"budget exceeded"}` {
				t.Fatalf("wrong conflict response: %s", w.Body.String())
			}
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
			tx.ID <= 0 ||
			tx.Category != category {
			t.Fatalf("wrong ID or category: %+v", tx)
		}

		ids[tx.ID] = true
	}

	if len(ids) != 12 ||
		len(s.ListTransactions()) != 12 {
		t.Fatal("concurrent budget check failed")
	}
}
