package main

import "errors"

// Transaction представляет финансовую транзакцию.
type Transaction struct {
	ID          int
	Amount      int    // Сумма в копейках: 150050 означает 1500 рублей 50 копеек.
	Category    string
	Description string
	Date        string // Дата в формате YYYY-MM-DD.
}

// Хранилище существует только до завершения процесса Ledger.
// В этой учебной версии функции вызываются последовательно из main.
var transactions = make([]Transaction, 0)

// AddTransaction проверяет сумму и сохраняет транзакцию с новым ID.
// Переданный вызывающим кодом ID заменяется автоматически.
func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("сумма транзакции не должна быть равна 0")
	}

	// Удаления нет, поэтому длина среза + 1 даёт уникальный ID.
	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)
	return nil
}

// ListTransactions возвращает копию среза, защищая хранилище от изменений снаружи.
func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}
