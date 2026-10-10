package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Transaction представляет финансовую транзакцию
type Transaction struct {
	ID          int
	Amount      int // Сумма в копейках.
	Category    string
	Description string
	Date        string // Формат YYYY-MM-DD.
}

// Validate проверяет поля и не изменяет объект или хранилище
// Метод не выполняет логирование.
func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("сумма транзакции должна быть положительной")
	}

	if strings.TrimSpace(tx.Category) == "" {
		return errors.New("категория транзакции не должна быть пустой")
	}

	if _, err := time.Parse("2006-01-02", tx.Date); err != nil {
		return fmt.Errorf(
			"дата должна быть в формате YYYY-MM-DD: %w",
			err,
		)
	}

	return nil
}

// Хранилище существует только во время работы процесса
// Вызовы функций в этой версии выполняются последовательно
var transactions = make([]Transaction, 0)

// AddTransaction сначала проверяет поля, затем бюджет,
// после чего сохраняет транзакцию с автоматически назначенным ID
func AddTransaction(tx Transaction) error {
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("транзакция: %w", err)
	}

	if budget, ok := budgets[tx.Category]; ok {
		var total int

		for _, saved := range transactions {
			if saved.Category == tx.Category {
				var err error

				total, err = addAmounts(total, saved.Amount)
				if err != nil {
					return err
				}
			}
		}

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

	// Хранилище меняется только после всех проверок
	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)

	return nil
}

// addAmounts защищает расчёт от переполнения int
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
func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)

	return result
}
