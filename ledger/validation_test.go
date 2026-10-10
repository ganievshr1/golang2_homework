package main

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		value   Validatable
		problem string
	}{
		{
			"valid transaction",
			Transaction{Amount: 100, Category: "еда", Date: "2026-10-10"},
			"",
		},
		{
			"zero amount",
			Transaction{Amount: 0, Category: "еда", Date: "2026-10-10"},
			"сумма",
		},
		{
			"negative amount",
			Transaction{Amount: -1, Category: "еда", Date: "2026-10-10"},
			"сумма",
		},
		{
			"empty transaction category",
			Transaction{Amount: 100, Date: "2026-10-10"},
			"категория",
		},
		{
			"blank transaction category",
			Transaction{Amount: 100, Category: " \t", Date: "2026-10-10"},
			"категория",
		},
		{
			"missing date",
			Transaction{Amount: 100, Category: "еда"},
			"дата",
		},
		{
			"wrong date format",
			Transaction{Amount: 100, Category: "еда", Date: "10.10.2026"},
			"дата",
		},
		{
			"impossible date",
			Transaction{Amount: 100, Category: "еда", Date: "2026-02-30"},
			"дата",
		},
		{
			"leap year",
			Transaction{Amount: 100, Category: "еда", Date: "2024-02-29"},
			"",
		},
		{
			"not leap year",
			Transaction{Amount: 100, Category: "еда", Date: "2025-02-29"},
			"дата",
		},
		{"valid budget", Budget{Category: "еда", Limit: 100}, ""},
		{"zero limit", Budget{Category: "еда", Limit: 0}, "лимит"},
		{"negative limit", Budget{Category: "еда", Limit: -1}, "лимит"},
		{"empty budget category", Budget{Limit: 100}, "категория"},
		{"blank budget category", Budget{Category: " \t", Limit: 100}, "категория"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckValid(tt.value)

			if tt.problem == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil ||
				!strings.Contains(err.Error(), tt.problem) {
				t.Fatalf(
					"error = %v, want description containing %q",
					err,
					tt.problem,
				)
			}
		})
	}
}

// Дополнительный тип показывает, что CheckValid
// работает не только с Transaction и Budget
type validationProbe struct {
	called bool
	err    error
}

func (p *validationProbe) Validate() error {
	p.called = true
	return p.err
}

func TestCheckValidDelegatesAndDoesNotStore(t *testing.T) {
	resetTransactions(t)

	cause := errors.New("validation failed")
	probe := &validationProbe{err: cause}

	if err := CheckValid(probe); err != cause || !probe.called {
		t.Fatalf("Validate was not delegated correctly: %v", err)
	}

	if err := CheckValid(nil); err == nil {
		t.Fatal("nil must fail")
	}

	for _, value := range []Validatable{
		testTransaction(100, "еда"),
		Budget{Category: "еда", Limit: 100},
	} {
		if err := CheckValid(value); err != nil {
			t.Fatal(err)
		}
	}

	if len(transactions) != 0 || len(budgets) != 0 {
		t.Fatal("validation must not save objects")
	}
}
