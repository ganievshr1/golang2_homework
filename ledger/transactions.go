package main

import (
	"errors"
	"fmt"
)

// Transaction представляет финансовую транзакцию
type Transaction struct {
	ID          int
	Amount      int // Сумма в копейках.
	Category    string
	Description string
	Date        string // Дата в формате YYYY-MM-DD.
}

// Хранилище существует только до завершения процесса
//
// В этой  версии функции вызываются последовательно
var transactions = make([]Transaction, 0)

// AddTransaction проверяет сумму и бюджет
// затем сохраняет транзакцию с новым ID
// Переданный вызывающим кодом ID заменяется автоматически
func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New(
			"сумма транзакции не должна быть равна 0",
		)
	}

	if budget, ok := budgets[tx.Category]; ok {
		var total int

		// Считаем сумму существующих транзакций категории
		for _, saved := range transactions {
			if saved.Category == tx.Category {
				var err error

				total, err = addAmounts(total, saved.Amount)
				if err != nil {
					return err
				}
			}
		}

		// Проверяем сумму с учётом новой транзакции
		nextTotal, err := addAmounts(total, tx.Amount)
		if err != nil {
			return err
		}

		if nextTotal > budget.Limit {
			return fmt.Errorf(
				"%w: категория %q, потрачено %d коп., новая сумма %d коп., лимит %d коп.",
				ErrBudgetExceeded,
				tx.Category,
				total,
				tx.Amount,
				budget.Limit,
			)
		}
	}

	// Изменяем хранилище только после всех проверок
	// Удаления записей нет, поэтому длина + 1 даёт уникальный ID
	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)

	return nil
}

// addAmounts защищает расчёт бюджета от переполнения int
func addAmounts(a, b int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1

	if (b > 0 && a > maxInt-b) || (b < 0 && a < minInt-b) {
		return 0, errors.New(
			"сумма транзакций категории выходит за диапазон int",
		)
	}

	return a + b, nil
}

// ListTransactions возвращает независимую копию списка
// Изменение возвращённого среза не изменяет хранилище
func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)

	return result
}
