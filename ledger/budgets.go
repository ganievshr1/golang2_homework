package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Budget struct {
	Category string `json:"category"`
	Limit    int    `json:"limit"` // Копейки.
}

var ErrBudgetExceeded = errors.New("budget exceeded")

func (b Budget) Validate() error {
	if b.Limit <= 0 {
		return errors.New(
			"лимит бюджета должен быть положительным",
		)
	}

	if int64(b.Limit) > maxMoney {
		return errors.New(
			"лимит превышает диапазон NUMERIC(14,2)",
		)
	}

	if strings.TrimSpace(b.Category) == "" {
		return errors.New(
			"категория бюджета не должна быть пустой",
		)
	}

	return nil
}

func SetBudget(b Budget) error {
	return setBudgets([]Budget{b})
}

// Одна SQL-транзакция сохраняет весь пакет
// или не сохраняет ничего.
func setBudgets(items []Budget) error {
	for _, b := range items {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("бюджет: %w", err)
		}
	}

	if len(items) == 0 {
		return nil
	}

	conn, err := database()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		operationTimeout,
	)
	defer cancel()

	tx, err := conn.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
		},
	)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Блокируем категории в одинаковом порядке.
	categories := make(map[string]bool)

	for _, b := range items {
		categories[b.Category] = true
	}

	names := make([]string, 0, len(categories))

	for category := range categories {
		names = append(names, category)
	}

	sort.Strings(names)

	for _, category := range names {
		if err := lockCategory(ctx, tx, category); err != nil {
			return err
		}
	}

	for _, b := range items {
		_, err := tx.ExecContext(
			ctx,
			"INSERT INTO budgets(category, limit_amount) VALUES($1, $2::numeric/100) ON CONFLICT(category) DO UPDATE SET limit_amount=EXCLUDED.limit_amount",
			b.Category,
			b.Limit,
		)
		if err != nil {
			return fmt.Errorf(
				"сохранение бюджета: %w",
				err,
			)
		}
	}

	return tx.Commit()
}

func ListBudgets() ([]Budget, error) {
	conn, err := database()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		operationTimeout,
	)
	defer cancel()

	rows, err := conn.QueryContext(
		ctx,
		"SELECT category, (limit_amount*100)::bigint FROM budgets ORDER BY category",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Budget, 0)

	for rows.Next() {
		var b Budget

		if err := rows.Scan(
			&b.Category,
			&b.Limit,
		); err != nil {
			return nil, err
		}

		result = append(result, b)
	}

	return result, rows.Err()
}

func LoadBudgets(r io.Reader) error {
	if r == nil {
		return errors.New(
			"загрузка бюджетов: источник чтения не задан",
		)
	}

	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var input []Budget

	if err := decoder.Decode(&input); err != nil {
		return fmt.Errorf(
			"загрузка бюджетов: чтение JSON: %w",
			err,
		)
	}

	if input == nil {
		return errors.New(
			"загрузка бюджетов: ожидается JSON-массив, а не null",
		)
	}

	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return fmt.Errorf(
				"загрузка бюджетов: данные после массива: %w",
				err,
			)
		}

		return errors.New(
			"загрузка бюджетов: лишнее JSON-значение",
		)
	}

	return setBudgets(input)
}
