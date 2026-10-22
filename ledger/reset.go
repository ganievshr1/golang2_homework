package ledger

// Reset очищает транзакции и бюджеты для изоляции тестов
// Вызывать только при отсутствии других обращений к Ledger:
// функция не синхронизирована с обработчиками Gateway
func Reset() {
	transactions = make([]Transaction, 0)
	budgets = make(map[string]Budget)
}
