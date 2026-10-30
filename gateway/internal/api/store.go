package api

import (
	"context"

	"example.com/finance-services/ledger"
)

type store interface {
	AddTransaction(ledger.Transaction) (ledger.Transaction, error)
	ListTransactions() ([]ledger.Transaction, error)
	SetBudget(ledger.Budget) error
	ListBudgets() ([]ledger.Budget, error)
	GetReportSummary(
		context.Context,
		string,
		string,
	) (ledger.ReportSummary, error)
}

type ledgerStore struct{}

func (ledgerStore) AddTransaction(
	tx ledger.Transaction,
) (ledger.Transaction, error) {
	return ledger.CreateTransaction(tx)
}

func (ledgerStore) ListTransactions() (
	[]ledger.Transaction,
	error,
) {
	return ledger.ListTransactions()
}

func (ledgerStore) SetBudget(b ledger.Budget) error {
	return ledger.SetBudget(b)
}

func (ledgerStore) ListBudgets() ([]ledger.Budget, error) {
	return ledger.ListBudgets()
}

func (ledgerStore) GetReportSummary(
	ctx context.Context,
	from, to string,
) (ledger.ReportSummary, error) {
	return ledger.GetReportSummary(ctx, from, to)
}
