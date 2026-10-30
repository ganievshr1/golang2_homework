package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Transaction struct {
	ID          int
	Amount      int // Копейки в Go и HTTP, рубли в PostgreSQL.
	Category    string
	Description string
	Date        string
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New(
			"сумма транзакции должна быть положительной",
		)
	}

	if int64(tx.Amount) > maxMoney {
		return errors.New(
			"сумма превышает диапазон NUMERIC(14,2)",
		)
	}

	if strings.TrimSpace(tx.Category) == "" {
		return errors.New(
			"категория транзакции не должна быть пустой",
		)
	}

	date, err := time.Parse("2006-01-02", tx.Date)
	if err != nil {
		return fmt.Errorf(
			"дата должна быть в формате YYYY-MM-DD: %w",
			err,
		)
	}

	if date.Year() < 1 {
		return errors.New(
			"год даты должен быть не меньше 0001",
		)
	}

	return nil
}

func AddTransaction(tx Transaction) error {
	_, err := CreateTransaction(tx)
	return err
}

// Возвращает созданную транзакцию с ID из INSERT RETURNING.
func CreateTransaction(item Transaction) (Transaction, error) {
	if err := item.Validate(); err != nil {
		return Transaction{}, fmt.Errorf(
			"транзакция: %w",
			err,
		)
	}

	conn, err := database()
	if err != nil {
		return Transaction{}, err
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
		return Transaction{}, err
	}
	defer tx.Rollback()

	if err := lockCategory(ctx, tx, item.Category); err != nil {
		return Transaction{}, err
	}

	var limit int64

	err = tx.QueryRowContext(
		ctx,
		"SELECT (limit_amount * 100)::bigint FROM budgets WHERE category=$1",
		item.Category,
	).Scan(&limit)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, err
	}

	if err == nil {
		// Расчёт выполняется в SQL NUMERIC,
		// чтобы не переполнить Go int при сложении.
		var exceeded bool

		err = tx.QueryRowContext(
			ctx,
			"SELECT COALESCE(SUM(amount), 0)*100 + $2::numeric > $3::numeric FROM expenses WHERE category=$1",
			item.Category,
			item.Amount,
			limit,
		).Scan(&exceeded)

		if err != nil {
			return Transaction{}, err
		}

		if exceeded {
			return Transaction{}, ErrBudgetExceeded
		}
	}

	err = tx.QueryRowContext(
		ctx,
		"INSERT INTO expenses(amount, category, description, date) VALUES($1::numeric/100, $2, $3, $4::date) RETURNING id",
		item.Amount,
		item.Category,
		item.Description,
		item.Date,
	).Scan(&item.ID)

	if err != nil {
		return Transaction{}, fmt.Errorf(
			"сохранение транзакции: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return Transaction{}, err
	}

	return item, nil
}

func ListTransactions() ([]Transaction, error) {
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
		"SELECT id, (amount*100)::bigint, category, COALESCE(description, ''), to_char(date, 'YYYY-MM-DD') FROM expenses ORDER BY date DESC, id DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Transaction, 0)

	for rows.Next() {
		var item Transaction

		if err := rows.Scan(
			&item.ID,
			&item.Amount,
			&item.Category,
			&item.Description,
			&item.Date,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, rows.Err()
}
