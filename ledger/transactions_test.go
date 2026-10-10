package main

import (
	"errors"
	"reflect"
	"testing"
)

func resetTransactions(t *testing.T) {
	t.Helper()

	oldTransactions, oldBudgets := transactions, budgets

	transactions = make([]Transaction, 0)
	budgets = make(map[string]Budget)

	t.Cleanup(func() {
		transactions, budgets = oldTransactions, oldBudgets
	})
}

func testTransaction(amount int, category string) Transaction {
	return Transaction{
		Amount:   amount,
		Category: category,
		Date:     "2026-10-10",
	}
}

func TestTransactionStorage(t *testing.T) {
	resetTransactions(t)

	if got := ListTransactions(); got == nil || len(got) != 0 {
		t.Fatal("expected empty list")
	}

	tx := testTransaction(100, "еда")
	tx.ID, tx.Description = 99, "Покупка"

	if err := AddTransaction(tx); err != nil {
		t.Fatal(err)
	}

	tx.ID = 1
	got := ListTransactions()

	if len(got) != 1 || got[0] != tx {
		t.Fatalf("saved: %+v", got)
	}

	got[0].Amount = 999

	if ListTransactions()[0] != tx {
		t.Fatal("list is not a copy")
	}
}

func TestInvalidTransactionDoesNotChangeStore(t *testing.T) {
	invalid := []Transaction{
		testTransaction(0, "еда"),
		testTransaction(-1, "еда"),
		testTransaction(100, ""),
		testTransaction(100, " \t"),
		{Amount: 100, Category: "еда"},
		{Amount: 100, Category: "еда", Date: "2026-02-30"},
	}

	for i, tx := range invalid {
		resetTransactions(t)

		if err := AddTransaction(testTransaction(100, "еда")); err != nil {
			t.Fatal(err)
		}

		before := ListTransactions()

		if err := AddTransaction(tx); err == nil {
			t.Fatalf("case %d accepted invalid data", i)
		}

		if !reflect.DeepEqual(before, ListTransactions()) {
			t.Fatal("store changed after rejection")
		}

		if err := AddTransaction(testTransaction(200, "еда")); err != nil {
			t.Fatal(err)
		}

		if ListTransactions()[1].ID != 2 {
			t.Fatal("rejection consumed ID")
		}
	}
}

func TestTransactionBudget(t *testing.T) {
	resetTransactions(t)

	if err := SetBudget(Budget{
		Category: "еда",
		Limit:    500000,
	}); err != nil {
		t.Fatal(err)
	}

	// Сумма двух операций точно достигает лимита.
	for _, amount := range []int{150050, 349950} {
		if err := AddTransaction(testTransaction(amount, "еда")); err != nil {
			t.Fatal(err)
		}
	}

	before := ListTransactions()

	if err := AddTransaction(testTransaction(1, "еда")); !errors.Is(
		err,
		ErrBudgetExceeded,
	) {
		t.Fatalf("want budget error, got %v", err)
	}

	if !reflect.DeepEqual(before, ListTransactions()) {
		t.Fatal("budget rejection changed store")
	}

	// Категория без бюджета.
	if err := AddTransaction(testTransaction(900000, "досуг")); err != nil {
		t.Fatal(err)
	}

	if got := ListTransactions(); len(got) != 3 || got[2].ID != 3 {
		t.Fatalf("saved: %+v", got)
	}
}

func TestTransactionOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)

	for _, lateBudget := range []bool{false, true} {
		resetTransactions(t)

		if !lateBudget {
			if err := SetBudget(Budget{
				Category: "еда",
				Limit:    maxInt,
			}); err != nil {
				t.Fatal(err)
			}
		}

		if err := AddTransaction(testTransaction(maxInt, "еда")); err != nil {
			t.Fatal(err)
		}

		// Проверяем также случай установки бюджета
		// после накопления больших сумм без ограничения
		if lateBudget {
			if err := AddTransaction(testTransaction(1, "еда")); err != nil {
				t.Fatal(err)
			}

			if err := SetBudget(Budget{
				Category: "еда",
				Limit:    maxInt,
			}); err != nil {
				t.Fatal(err)
			}
		}

		before := ListTransactions()

		if err := AddTransaction(testTransaction(1, "еда")); err == nil {
			t.Fatal("overflow accepted")
		}

		if !reflect.DeepEqual(before, ListTransactions()) {
			t.Fatal("overflow changed store")
		}
	}
}
