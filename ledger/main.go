package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	if err := runLedger("budgets.json", os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// runLedger содержит сценарий main,
// чтобы его можно было проверить автоматическими тестами
func runLedger(budgetFile string, out io.Writer) error {
	fmt.Fprintln(out, "Ledger service started")

	file, err := os.Open(budgetFile)
	if err != nil {
		return fmt.Errorf(
			"не удалось открыть файл бюджетов %q: %w",
			budgetFile,
			err,
		)
	}
	defer file.Close()

	if err := LoadBudgets(bufio.NewReader(file)); err != nil {
		return fmt.Errorf(
			"файл бюджетов %q: %w",
			budgetFile,
			err,
		)
	}

	fmt.Fprintf(
		out,
		"Бюджеты загружены из %s\n",
		budgetFile,
	)

	// Демонстрация добавления бюджета.
	SetBudget(Budget{
		Category: "Обучение",
		Limit:    30000,
	})

	// Демонстрация обновления бюджета.
	SetBudget(Budget{
		Category: "Обучение",
		Limit:    50000,
	})

	examples := []Transaction{
		{
			Amount:      150050,
			Category:    "Продукты",
			Description: "Покупка в магазине",
			Date:        "2026-10-03",
		},
		{
			Amount:      6500,
			Category:    "Транспорт",
			Description: "Проезд в метро",
			Date:        "2026-10-03",
		},
		{
			Amount:      39900,
			Category:    "Обучение",
			Description: "Покупка книги после увеличения лимита",
			Date:        "2026-10-03",
		},
		// 150050 + 400000 > 500000: ожидается отказ.
		{
			Amount:      400000,
			Category:    "Продукты",
			Description: "Покупка сверх бюджета",
			Date:        "2026-10-03",
		},
		// 150050 + 349950 = 500000: операция допустима.
		{
			Amount:      349950,
			Category:    "Продукты",
			Description: "Покупка в пределах остатка бюджета",
			Date:        "2026-10-03",
		},
		{
			Amount:      -5000,
			Category:    "Продукты",
			Description: "Возврат покупки",
			Date:        "2026-10-03",
		},
		{
			Amount:      5000,
			Category:    "Продукты",
			Description: "Покупка на сумму возврата",
			Date:        "2026-10-03",
		},
		{
			Amount:      100000,
			Category:    "Досуг",
			Description: "Категория без бюджета",
			Date:        "2026-10-03",
		},
		{
			Amount:      0,
			Category:    "Проверка",
			Description: "Недопустимая нулевая сумма",
		},
	}

	for _, tx := range examples {
		if err := AddTransaction(tx); err != nil {
			fmt.Fprintf(
				out,
				"Отказ: %s: %v\n",
				tx.Description,
				err,
			)
			continue
		}

		fmt.Fprintf(
			out,
			"Добавлено: %s (%d коп., %s)\n",
			tx.Description,
			tx.Amount,
			tx.Category,
		)
	}

	all := ListTransactions()

	fmt.Fprintf(
		out,
		"\nСохранено транзакций: %d\n",
		len(all),
	)

	for _, tx := range all {
		fmt.Fprintf(
			out,
			"ID: %d | Сумма: %d коп. | Категория: %s | Описание: %s | Дата: %s\n",
			tx.ID,
			tx.Amount,
			tx.Category,
			tx.Description,
			tx.Date,
		)
	}

	return nil
}
