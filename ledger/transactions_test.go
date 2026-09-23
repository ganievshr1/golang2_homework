package main

import "testing"

func resetTransactions(t *testing.T) {
	t.Helper()
	previous := transactions
	transactions = make([]Transaction, 0)
	t.Cleanup(func() { transactions = previous })
}

func TestListTransactionsInitiallyEmpty(t *testing.T) {
	resetTransactions(t)
	if got := ListTransactions(); got == nil || len(got) != 0 {
		t.Fatalf("initial list = %v, want non-nil empty slice", got)
	}
}

func TestAddTransactionAndList(t *testing.T) {
	resetTransactions(t)
	examples := []Transaction{
		{ID: 99, Amount: 150050, Category: "Продукты", Description: "Магазин", Date: "2026-09-21"},
		{ID: 99, Amount: 6500, Category: "Транспорт", Description: "Метро", Date: "2026-09-22"},
		{ID: 99, Amount: -5000, Category: "Возврат", Description: "Возврат покупки", Date: "2026-09-23"},
	}
	for _, tx := range examples {
		if err := AddTransaction(tx); err != nil {
			t.Fatalf("AddTransaction() error = %v", err)
		}
	}

	got := ListTransactions()
	if len(got) != len(examples) {
		t.Fatalf("transaction count = %d, want %d", len(got), len(examples))
	}
	for i, expected := range examples {
		expected.ID = i + 1
		if got[i] != expected {
			t.Errorf("transaction[%d] = %+v, want %+v", i, got[i], expected)
		}
	}
}

func TestZeroAmountDoesNotChangeStoreOrConsumeID(t *testing.T) {
	resetTransactions(t)
	first := Transaction{Amount: 100, Category: "Продукты"}
	if err := AddTransaction(first); err != nil {
		t.Fatal(err)
	}
	if err := AddTransaction(Transaction{Amount: 0}); err == nil {
		t.Fatal("zero amount must return an error")
	}
	got := ListTransactions()
	first.ID = 1
	if len(got) != 1 || got[0] != first {
		t.Fatalf("store changed after rejected transaction: %+v", got)
	}
	if err := AddTransaction(Transaction{Amount: 200}); err != nil {
		t.Fatal(err)
	}
	if id := ListTransactions()[1].ID; id != 2 {
		t.Fatalf("next ID = %d, want 2", id)
	}
}

func TestListTransactionsReturnsCopy(t *testing.T) {
	resetTransactions(t)
	original := Transaction{Amount: 100, Category: "Продукты"}
	if err := AddTransaction(original); err != nil {
		t.Fatal(err)
	}
	listed := ListTransactions()
	listed[0].Amount = 999
	listed[0].Category = "Изменено"
	original.ID = 1
	if got := ListTransactions()[0]; got != original {
		t.Fatalf("changing returned slice changed the store: %+v", got)
	}
}
