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

func TestSetBudget(t *testing.T) {
	resetTransactions(t)
	budgets = nil

	for _, b := range []Budget{
		{Category: "еда", Limit: 100},
		{Category: "проезд", Limit: 100},
	} {
		if err := SetBudget(b); err != nil {
			t.Fatal(err)
		}

		if err := AddTransaction(testTransaction(100, b.Category)); err != nil {
			t.Fatal(err)
		}
	}

	before := ListTransactions()

	if err := SetBudget(Budget{Category: "еда", Limit: 200}); err != nil {
		t.Fatal(err)
	}

	if len(budgets) != 2 || budgets["еда"].Limit != 200 {
		t.Fatal("budget was not updated")
	}

	if !reflect.DeepEqual(before, ListTransactions()) {
		t.Fatal("update changed history")
	}

	if err := AddTransaction(testTransaction(100, "еда")); err != nil {
		t.Fatal(err)
	}

	if err := SetBudget(Budget{Category: "еда", Limit: 50}); err != nil {
		t.Fatal(err)
	}

	if err := AddTransaction(testTransaction(1, "еда")); !errors.Is(
		err,
		ErrBudgetExceeded,
	) {
		t.Fatalf("lowered budget was ignored: %v", err)
	}
}

func TestInvalidBudgetDoesNotChangeStore(t *testing.T) {
	resetTransactions(t)

	if err := SetBudget(Budget{
		Category: "еда",
		Limit:    500000,
	}); err != nil {
		t.Fatal(err)
	}

	for _, b := range []Budget{
		{Category: "еда", Limit: 0},
		{Category: "еда", Limit: -1},
		{Category: "новая", Limit: 0},
		{Category: "", Limit: 100},
		{Category: " \t", Limit: 100},
	} {
		if err := SetBudget(b); err == nil {
			t.Fatalf("accepted invalid budget: %+v", b)
		}

		if len(budgets) != 1 || budgets["еда"].Limit != 500000 {
			t.Fatal("invalid budget changed store")
		}
	}
}

func TestLoadBudgets(t *testing.T) {
	resetTransactions(t)

	if err := SetBudget(Budget{
		Category: "связь",
		Limit:    50000,
	}); err != nil {
		t.Fatal(err)
	}

	data := `[
		{"category":"еда","limit":100},
		{"category":"еда","limit":200},
		{"category":"проезд","limit":300}
	]`

	if err := LoadBudgets(strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}

	want := map[string]Budget{
		"еда":    {Category: "еда", Limit: 200},
		"проезд": {Category: "проезд", Limit: 300},
		"связь":  {Category: "связь", Limit: 50000},
	}

	if !reflect.DeepEqual(budgets, want) {
		t.Fatalf("loaded: %+v", budgets)
	}

	if err := LoadBudgets(strings.NewReader("[]")); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(budgets, want) {
		t.Fatal("empty array changed store")
	}
}

func TestLoadBudgetsRejectsInvalidInput(t *testing.T) {
	inputs := []string{
		``,
		`null`,
		`{}`,
		`[`,
		`[null]`,
		`[{"category":"еда"}]`,
		`[{"category":"еда","limit":null}]`,
		`[{"category":"еда","limit":0}]`,
		`[{"category":"еда","limit":-1}]`,
		`[{"category":"еда","limit":1.5}]`,
		`[{"category":"еда","limit":"100"}]`,
		`[{"category":"еда","limit":100,"extra":true}]`,
		`[{"category":" ","limit":100}]`,
		`[{"limit":100}]`,
		`[{"category":"еда","limit":100}] {}`,
		`[{"category":"еда","limit":100}] broken`,
		`[{"category":"еда","limit":100},{"category":"проезд","limit":0}]`,
	}

	for _, data := range inputs {
		t.Run(data, func(t *testing.T) {
			resetTransactions(t)

			if err := SetBudget(Budget{
				Category: "еда",
				Limit:    500000,
			}); err != nil {
				t.Fatal(err)
			}

			if err := LoadBudgets(strings.NewReader(data)); err == nil {
				t.Fatal("invalid JSON accepted")
			}

			if len(budgets) != 1 || budgets["еда"].Limit != 500000 {
				t.Fatal("partial update after error")
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

func TestLoadBudgetsReadErrors(t *testing.T) {
	resetTransactions(t)
	cause := errors.New("read failed")

	for _, reader := range []io.Reader{
		failingReader{cause},
		io.MultiReader(
			strings.NewReader(`[{"category":"еда","limit":100}]`),
			failingReader{cause},
		),
	} {
		if err := LoadBudgets(reader); !errors.Is(err, cause) {
			t.Fatalf("lost read error: %v", err)
		}

		if len(budgets) != 0 {
			t.Fatal("read failure changed store")
		}
	}

	if err := LoadBudgets(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
}

func TestMainScenario(t *testing.T) {
	resetTransactions(t)

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

	if len(all) != 5 || strings.Count(output.String(), "Отказ:") != 4 {
		t.Fatalf("unexpected demo:\n%s", output.String())
	}

	for i, tx := range all {
		if err := tx.Validate(); err != nil {
			t.Fatal(err)
		}

		if tx.ID != i+1 || tx.Description == "Сверх бюджета" {
			t.Fatalf("saved: %+v", tx)
		}
	}

	for _, fragment := range []string{
		"CheckValid(main.Transaction): OK",
		"CheckValid(main.Budget): OK",
		"CheckValid(main.Transaction): ошибка:",
		"CheckValid(main.Budget): ошибка:",
		"budget exceeded",
	} {
		if !strings.Contains(output.String(), fragment) {
			t.Fatalf(
				"missing %q:\n%s",
				fragment,
				output.String(),
			)
		}
	}
}

func TestMainScenarioFileErrors(t *testing.T) {
	for _, name := range []string{
		"budgets.json",
		`windows\path\budgets.json`,
	} {
		t.Run(name, func(t *testing.T) {
			resetTransactions(t)

			path := filepath.Join(t.TempDir(), name)

			if err := runLedger(path, io.Discard); !errors.Is(
				err,
				os.ErrNotExist,
			) {
				t.Fatalf("missing file: %v", err)
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

			var syntaxError *json.SyntaxError
			if !errors.As(err, &syntaxError) {
				t.Fatalf("expected JSON syntax error: %v", err)
			}

			if !strings.Contains(err.Error(), fmt.Sprintf("%q", path)) {
				t.Fatalf("missing file path: %v", err)
			}

			if len(transactions) != 0 || len(budgets) != 0 {
				t.Fatal("file error changed store")
			}
		})
	}
}
