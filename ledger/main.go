package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Ledger service started")

	examples := []Transaction{
		{Amount: 150050, Category: "Продукты", Description: "Покупка в магазине", Date: "2026-09-21"},
		{Amount: 6500, Category: "Транспорт", Description: "Проезд в метро", Date: "2026-09-22"},
		{Amount: 39900, Category: "Обучение", Description: "Покупка книги", Date: "2026-09-23"},
	}

	for _, tx := range examples {
		if err := AddTransaction(tx); err != nil {
			log.Fatalf("Не удалось добавить транзакцию: %v", err)
		}
	}

	// Ошибочная транзакция не должна попасть в хранилище.
	if err := AddTransaction(Transaction{Amount: 0, Category: "Проверка"}); err != nil {
		fmt.Printf("Ожидаемая ошибка: %v\n", err)
	}

	all := ListTransactions()
	fmt.Printf("\nСохранено транзакций: %d\n", len(all))
	for _, tx := range all {
		fmt.Printf("ID: %d | Сумма: %d коп. | Категория: %s | Описание: %s | Дата: %s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
