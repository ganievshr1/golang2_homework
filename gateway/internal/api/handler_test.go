package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/finance-services/ledger"
)

// Заглушка проверяет HTTP-контракт.
// SQL и Redis проверяются в Ledger.
type fakeStore struct {
	err          error
	saved        ledger.Transaction
	budget       ledger.Budget
	transactions []ledger.Transaction
	budgets      []ledger.Budget
	calls        int
}

func (s *fakeStore) AddTransaction(
	tx ledger.Transaction,
) (ledger.Transaction, error) {
	s.calls++
	tx.ID = 42
	s.saved = tx

	return tx, s.err
}

func (s *fakeStore) SetBudget(b ledger.Budget) error {
	s.calls++
	s.budget = b

	return s.err
}

func (s *fakeStore) ListTransactions() (
	[]ledger.Transaction,
	error,
) {
	return s.transactions, s.err
}

func (s *fakeStore) ListBudgets() ([]ledger.Budget, error) {
	return s.budgets, s.err
}

func (s *fakeStore) GetReportSummary(
	context.Context,
	string,
	string,
) (ledger.ReportSummary, error) {
	return ledger.ReportSummary{
		From:  "2026-10-01",
		To:    "2026-10-31",
		Total: 450,
		Items: []ledger.ReportItem{
			{Category: "еда", Amount: 450},
		},
	}, s.err
}

func request(
	t *testing.T,
	h http.Handler,
	method, path, body string,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()

	h.ServeHTTP(
		w,
		httptest.NewRequest(
			method,
			path,
			strings.NewReader(body),
		),
	)

	if w.Code != status {
		t.Fatalf(
			"%s %s: status=%d body=%s",
			method,
			path,
			w.Code,
			w.Body,
		)
	}

	if w.Header().Get("Content-Type") !=
		"application/json; charset=utf-8" ||
		!json.Valid(w.Body.Bytes()) {
		t.Fatal("invalid JSON/header")
	}

	if status >= 400 {
		var result map[string]string

		if json.Unmarshal(w.Body.Bytes(), &result) != nil ||
			len(result) != 1 ||
			result["error"] == "" {
			t.Fatalf("invalid error: %s", w.Body)
		}
	}

	return w
}

func TestHTTPContracts(t *testing.T) {
	s := &fakeStore{}
	var logs bytes.Buffer

	h := newHandler(s, log.New(&logs, "", 0))

	for _, path := range []string{
		"/api/transactions",
		"/api/budgets",
	} {
		w := request(t, h, "GET", path, "", 200)

		if strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal("want empty array")
		}
	}

	body := `{"amount":450,"category":"еда","description":"ланч","date":"2026-10-30"}`

	w := request(
		t,
		h,
		"POST",
		"/api/transactions",
		body,
		201,
	)

	var saved TransactionResponse

	if err := json.Unmarshal(
		w.Body.Bytes(),
		&saved,
	); err != nil {
		t.Fatal(err)
	}

	if saved.ID != 42 ||
		saved.Amount != 450 ||
		saved.Category != "еда" ||
		saved.Description != "ланч" ||
		saved.Date != "2026-10-30" {
		t.Fatalf("wrong DTO: %+v", saved)
	}

	w = request(
		t,
		h,
		"POST",
		"/api/budgets",
		`{"category":"еда","limit":1000}`,
		201,
	)

	var b BudgetResponse

	if err := json.Unmarshal(
		w.Body.Bytes(),
		&b,
	); err != nil {
		t.Fatal(err)
	}

	if b.Category != "еда" ||
		b.Limit != 1000 ||
		s.budget.Limit != 1000 {
		t.Fatalf("wrong budget DTO: %+v", b)
	}

	s.transactions = []ledger.Transaction{s.saved}
	s.budgets = []ledger.Budget{s.budget}

	w = request(
		t,
		h,
		"GET",
		"/api/transactions",
		"",
		200,
	)

	var txs []TransactionResponse

	if err := json.Unmarshal(
		w.Body.Bytes(),
		&txs,
	); err != nil || len(txs) != 1 || txs[0] != saved {
		t.Fatal("wrong transactions list")
	}

	w = request(
		t,
		h,
		"GET",
		"/api/budgets",
		"",
		200,
	)

	var budgets []BudgetResponse

	if err := json.Unmarshal(
		w.Body.Bytes(),
		&budgets,
	); err != nil || len(budgets) != 1 || budgets[0] != b {
		t.Fatal("wrong budgets list")
	}

	request(
		t,
		h,
		"GET",
		"/api/reports/summary?from=2026-10-01&to=2026-10-31",
		"",
		200,
	)

	request(t, h, "GET", "/missing", "", 404)
	request(t, h, "DELETE", "/api/budgets", "", 405)
	request(t, h, "POST", "/api/reports/summary", "", 405)

	for _, fragment := range []string{
		"method=GET",
		"method=POST",
		`path="/api/transactions"`,
		"duration=",
	} {
		if !strings.Contains(logs.String(), fragment) {
			t.Fatalf("missing log %s", fragment)
		}
	}
}

func TestInvalidHTTP(t *testing.T) {
	s := &fakeStore{}
	h := newHandler(s, nil)

	for _, path := range []string{
		"/api/transactions",
		"/api/budgets",
	} {
		for _, body := range []string{
			"",
			"null",
			"[]",
			"{",
			"{}",
			`{"unknown":1}`,
			"{} {}",
			"{} broken",
			strings.Repeat(" ", maxBodyBytes+1),
		} {
			request(t, h, "POST", path, body, 400)
		}
	}

	for _, body := range []string{
		`{"amount":0,"category":"еда","date":"2026-10-30"}`,
		`{"amount":-1,"category":"еда","date":"2026-10-30"}`,
		`{"amount":1.5,"category":"еда","date":"2026-10-30"}`,
		`{"amount":1,"category":" ","date":"2026-10-30"}`,
		`{"amount":1,"category":"еда","date":"2026-02-30"}`,
	} {
		request(
			t,
			h,
			"POST",
			"/api/transactions",
			body,
			400,
		)
	}

	for _, body := range []string{
		`{"category":"еда","limit":0}`,
		`{"category":"еда","limit":-1}`,
		`{"category":" ","limit":100}`,
	} {
		request(
			t,
			h,
			"POST",
			"/api/budgets",
			body,
			400,
		)
	}

	if s.calls != 0 {
		t.Fatal("invalid request reached storage")
	}
}

func TestHTTPErrors(t *testing.T) {
	s := &fakeStore{
		err: errors.New("private SQL error"),
	}

	var logs bytes.Buffer
	h := newHandler(s, log.New(&logs, "", 0))

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{
			"GET",
			"/api/transactions",
			"",
		},
		{
			"GET",
			"/api/budgets",
			"",
		},
		{
			"GET",
			"/api/reports/summary?from=2026-10-01&to=2026-10-31",
			"",
		},
		{
			"POST",
			"/api/transactions",
			`{"amount":1,"category":"еда","date":"2026-10-30"}`,
		},
		{
			"POST",
			"/api/budgets",
			`{"category":"еда","limit":100}`,
		},
	}

	for _, tc := range tests {
		w := request(
			t,
			h,
			tc.method,
			tc.path,
			tc.body,
			500,
		)

		if strings.TrimSpace(w.Body.String()) !=
			`{"error":"internal server error"}` {
			t.Fatal("SQL detail leaked")
		}
	}

	if !strings.Contains(
		logs.String(),
		"private SQL error",
	) {
		t.Fatal("error not logged")
	}

	s.err = fmt.Errorf(
		"wrapped: %w",
		ledger.ErrBudgetExceeded,
	)

	w := request(
		t,
		h,
		"POST",
		"/api/transactions",
		`{"amount":1,"category":"еда","date":"2026-10-30"}`,
		409,
	)

	if strings.TrimSpace(w.Body.String()) !=
		`{"error":"budget exceeded"}` {
		t.Fatal("wrong conflict")
	}

	s.err = ledger.ErrInvalidPeriod

	request(
		t,
		h,
		"GET",
		"/api/reports/summary?from=bad&to=bad",
		"",
		400,
	)
}
