package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddTransactionWithBudget(t *testing.T) {
	tests := []struct {
		name     string
		category string
		amounts  []int
		wantErr  bool
	}{
		{"below limit", "Продукты", []int{150050}, false},
		{"exact limit", "Продукты", []int{150050, 349950}, false},
		{"single exceeds limit", "Продукты", []int{500001}, true},
		{"sum exceeds limit", "Продукты", []int{150050, 400000}, true},
		{"refund frees budget", "Продукты", []int{500000, -5000, 5000}, false},
		{"no budget", "Досуг", []int{900000}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTransactions(t)

			SetBudget(Budget{
				Category: "Продукты",
				Limit:    500000,
			})

			for i, amount := range tt.amounts {
				before := ListTransactions()

				err := AddTransaction(Transaction{
					Amount:   amount,
					Category: tt.category,
				})

				if tt.wantErr && i == len(tt.amounts)-1 {
					if !errors.Is(err, ErrBudgetExceeded) {
						t.Fatalf(
							"error = %v, want ErrBudgetExceeded",
							err,
						)
					}

					if !reflect.DeepEqual(before, ListTransactions()) {
						t.Fatal("rejected transaction changed the store")
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}

			// Отказ не должен расходовать ID.
			nextID := len(ListTransactions()) + 1

			if err := AddTransaction(Transaction{
				Amount:   1,
				Category: "Без лимита",
			}); err != nil {
				t.Fatal(err)
			}

			if got := ListTransactions()[nextID-1].ID; got != nextID {
				t.Fatalf("ID = %d, want %d", got, nextID)
			}
		})
	}
}

func TestSetBudgetUpdatesLimitAndKeepsHistory(t *testing.T) {
	resetTransactions(t)

	budgets = nil

	SetBudget(Budget{
		Category: "еда",
		Limit:    100,
	})

	if err := AddTransaction(Transaction{
		Amount:   100,
		Category: "еда",
	}); err != nil {
		t.Fatal(err)
	}

	SetBudget(Budget{
		Category: "еда",
		Limit:    200,
	})

	if err := AddTransaction(Transaction{
		Amount:   100,
		Category: "еда",
	}); err != nil {
		t.Fatal(err)
	}

	SetBudget(Budget{
		Category: "еда",
		Limit:    50,
	})

	if err := AddTransaction(Transaction{
		Amount:   1,
		Category: "еда",
	}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}

	if len(ListTransactions()) != 2 || len(budgets) != 1 {
		t.Fatal(
			"budget update changed history or created duplicate categories",
		)
	}
}

func TestBudgetsAreIndependentAndZeroIsAllowed(t *testing.T) {
	resetTransactions(t)

	SetBudget(Budget{Category: "еда", Limit: 100})
	SetBudget(Budget{Category: "транспорт", Limit: 100})
	SetBudget(Budget{Category: "покупки", Limit: 0})

	for _, category := range []string{"еда", "транспорт"} {
		if err := AddTransaction(Transaction{
			Amount:   100,
			Category: category,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := AddTransaction(Transaction{
		Amount:   1,
		Category: "покупки",
	}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("zero budget error = %v", err)
	}
}

func TestBudgetRejectsIntegerOverflow(t *testing.T) {
	resetTransactions(t)

	maxInt := int(^uint(0) >> 1)

	SetBudget(Budget{
		Category: "еда",
		Limit:    maxInt,
	})

	if err := AddTransaction(Transaction{
		Amount:   maxInt,
		Category: "еда",
	}); err != nil {
		t.Fatal(err)
	}

	if err := AddTransaction(Transaction{
		Amount:   1,
		Category: "еда",
	}); err == nil {
		t.Fatal("integer overflow must be rejected")
	}

	if len(ListTransactions()) != 1 {
		t.Fatal("overflow changed the store")
	}
}

func TestLoadBudgets(t *testing.T) {
	resetTransactions(t)

	SetBudget(Budget{
		Category: "связь",
		Limit:    50000,
	})

	input := `[
		{"category":"еда","limit":500000},
		{"category":"еда","limit":600000},
		{"category":"транспорт","limit":0}
	]`

	if err := LoadBudgets(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}

	want := map[string]Budget{
		"еда":       {Category: "еда", Limit: 600000},
		"транспорт": {Category: "транспорт", Limit: 0},
		"связь":     {Category: "связь", Limit: 50000},
	}

	if !reflect.DeepEqual(budgets, want) {
		t.Fatalf("budgets = %v, want %v", budgets, want)
	}

	if err := LoadBudgets(strings.NewReader("[]")); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(budgets, want) {
		t.Fatal("empty array must preserve budgets")
	}
}

func TestLoadBudgetsRejectsInvalidInputAtomically(t *testing.T) {
	inputs := []string{
		``,
		`null`,
		`{}`,
		`[`,
		`[null]`,
		`[{"category":"еда"}]`,
		`[{"category":"еда","limit":null}]`,
		`[{"category":"еда","limit":-1}]`,
		`[{"category":"еда","limit":1.5}]`,
		`[{"category":"еда","limit":"100"}]`,
		`[{"category":"еда","limit":100,"extra":true}]`,
		`[{"category":" ","limit":100}]`,
		`[{"limit":100}]`,
		`[{"category":"еда","limit":100}] {}`,
		`[{"category":"еда","limit":100}] broken`,
		`[{"category":"еда","limit":100},{"category":"транспорт","limit":-1}]`,
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			resetTransactions(t)

			SetBudget(Budget{
				Category: "еда",
				Limit:    500000,
			})

			if err := LoadBudgets(strings.NewReader(input)); err == nil {
				t.Fatal("invalid JSON must return an error")
			}

			if len(budgets) != 1 || budgets["еда"].Limit != 500000 {
				t.Fatal("failed load partially changed budgets")
			}
		})
	}
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestLoadBudgetsReaderErrors(t *testing.T) {
	resetTransactions(t)

	cause := errors.New("read failed")

	readers := []io.Reader{
		failingReader{err: cause},
		io.MultiReader(
			strings.NewReader(`[{"category":"еда","limit":100}]`),
			failingReader{err: cause},
		),
	}

	for _, reader := range readers {
		if err := LoadBudgets(reader); !errors.Is(err, cause) {
			t.Fatalf(
				"error = %v, want wrapped read failure",
				err,
			)
		}

		if len(budgets) != 0 {
			t.Fatal("read failure changed budgets")
		}
	}

	if err := LoadBudgets(nil); err == nil {
		t.Fatal("nil reader must return an error")
	}
}

func TestMainScenario(t *testing.T) {
	resetTransactions(t)

	// Тест создаёт собственный файл бюджетов.
	// Изменения рабочего budgets.json не влияют на результат.
	path := filepath.Join(t.TempDir(), "budgets.json")

	data := `[
		{"category":"Продукты","limit":500000},
		{"category":"Транспорт","limit":200000}
	]`

	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer

	if err := runLedger(path, &output); err != nil {
		t.Fatal(err)
	}

	all := ListTransactions()

	if len(all) != 7 {
		t.Fatalf(
			"saved %d transactions, want 7\nLedger output:\n%s",
			len(all),
			output.String(),
		)
	}

	for _, tx := range all {
		if tx.Amount == 0 ||
			tx.Description == "Покупка сверх бюджета" {
			t.Fatalf(
				"rejected transaction was saved: %+v",
				tx,
			)
		}
	}

	if !strings.Contains(output.String(), "budget exceeded") ||
		strings.Count(output.String(), "Отказ:") != 2 {
		t.Fatalf(
			"missing refusal messages:\n%s",
			output.String(),
		)
	}
}

func TestMainScenarioFileErrors(t *testing.T) {
	names := []string{
		"budgets.json",
		`windows\path\budgets.json`,
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			resetTransactions(t)

			path := filepath.Join(t.TempDir(), name)

			if err := runLedger(path, io.Discard); !errors.Is(
				err,
				os.ErrNotExist,
			) {
				t.Fatalf("missing file error = %v", err)
			}

			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(
				path,
				[]byte("broken JSON"),
				0600,
			); err != nil {
				t.Fatal(err)
			}

			err := runLedger(path, io.Discard)

			// Проверяем настоящую причину ошибки.
			var syntaxError *json.SyntaxError

			if !errors.As(err, &syntaxError) {
				t.Fatalf(
					"error = %v, want wrapped JSON syntax error",
					err,
				)
			}

			quotedPath := fmt.Sprintf("%q", path)

			if !strings.Contains(err.Error(), quotedPath) {
				t.Fatalf(
					"error does not contain quoted file path: %v",
					err,
				)
			}

			if len(budgets) != 0 || len(ListTransactions()) != 0 {
				t.Fatal("file error changed the store")
			}
		})
	}
}
