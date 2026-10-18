package api

import (
	"sync"

	"example.com/finance-services/ledger"
)

// store отделяет HTTP от вызовов Ledger
// и позволяет воспроизводить внутренние ошибки в тестах.
type store interface {
	AddTransaction(ledger.Transaction) (ledger.Transaction, error)
	ListTransactions() []ledger.Transaction
	SetBudget(ledger.Budget) error
	ListBudgets() []ledger.Budget
}

type ledgerStore struct{}

// Ledger использует глобальные хранилища,
// поэтому блокировка общая для всех обработчиков.
var ledgerMu sync.Mutex

func (ledgerStore) AddTransaction(
	tx ledger.Transaction,
) (ledger.Transaction, error) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()

	if err := ledger.AddTransaction(tx); err != nil {
		return ledger.Transaction{}, err
	}

	all := ledger.ListTransactions()
	return all[len(all)-1], nil
}

func (ledgerStore) ListTransactions() []ledger.Transaction {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()

	return ledger.ListTransactions()
}

func (ledgerStore) SetBudget(b ledger.Budget) error {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()

	return ledger.SetBudget(b)
}

func (ledgerStore) ListBudgets() []ledger.Budget {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()

	return ledger.ListBudgets()
}
