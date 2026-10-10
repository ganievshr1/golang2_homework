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

func runLedger(budgetFile string, out io.Writer) error {
	fmt.Fprintln(out, "Ledger service started")

	// Один интерфейс — разные типы
	// CheckValid только проверяет данные и ничего не сохраняет
	checks := []Validatable{
		Transaction{
			Amount:   10000,
			Category: "Продукты",
			Date:     "2026-10-10",
		},
		Budget{
			Category: "Продукты",
			Limit:    500000,
		},
		Transaction{
			Amount:   -100,
			Category: "Продукты",
			Date:     "2026-10-10",
		},
		Budget{
			Category: " ",
			Limit:    500000,
		},
	}

	for _, value := range checks {
		if err := CheckValid(value); err != nil {
			fmt.Fprintf(
				out,
				"CheckValid(%T): ошибка: %v\n",
				value,
				err,
			)
		} else {
			fmt.Fprintf(out, "CheckValid(%T): OK\n", value)
		}
	}

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

	// Добавление бюджета, затем его обновление
	for _, limit := range []int{30000, 50000} {
		if err := SetBudget(Budget{
			Category: "Обучение",
			Limit:    limit,
		}); err != nil {
			return err
		}
	}

	examples := []Transaction{
		{
			Amount:      150050,
			Category:    "Продукты",
			Description: "Магазин",
			Date:        "2026-10-10",
		},
		{
			Amount:      6500,
			Category:    "Транспорт",
			Description: "Метро",
			Date:        "2026-10-10",
		},
		{
			Amount:      39900,
			Category:    "Обучение",
			Description: "Книга",
			Date:        "2026-10-10",
		},
		{
			Amount:      400000,
			Category:    "Продукты",
			Description: "Сверх бюджета",
			Date:        "2026-10-10",
		},
		{
			Amount:      349950,
			Category:    "Продукты",
			Description: "До точного лимита",
			Date:        "2026-10-10",
		},
		{
			Amount:      100000,
			Category:    "Досуг",
			Description: "Без бюджета",
			Date:        "2026-10-10",
		},
		{
			Amount:      -5000,
			Category:    "Продукты",
			Description: "Отрицательная сумма",
			Date:        "2026-10-10",
		},
		{
			Amount:      100,
			Category:    " ",
			Description: "Пустая категория",
			Date:        "2026-10-10",
		},
		{
			Amount:      100,
			Category:    "Досуг",
			Description: "Неверная дата",
			Date:        "2026-02-30",
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

		fmt.Fprintf(out, "Добавлено: %s\n", tx.Description)
	}

	all := ListTransactions()
	fmt.Fprintf(out, "Сохранено транзакций: %d\n", len(all))

	for _, tx := range all {
		fmt.Fprintf(
			out,
			"ID: %d | %d коп. | %s | %s | %s\n",
			tx.ID,
			tx.Amount,
			tx.Category,
			tx.Description,
			tx.Date,
		)
	}

	return nil
}
