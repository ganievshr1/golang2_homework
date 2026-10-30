package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

const reportTTL = 30 * time.Second

var ErrInvalidPeriod = errors.New(
	"некорректный период отчёта",
)

type ReportItem struct {
	Category string `json:"category"`
	Amount   int64  `json:"amount"` // Копейки.
}

type ReportSummary struct {
	From  string       `json:"from"`
	To    string       `json:"to"`
	Total int64        `json:"total"`
	Items []ReportItem `json:"items"`
}

func GetReportSummary(
	ctx context.Context,
	from, to string,
) (ReportSummary, error) {
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)

	if e1 != nil ||
		e2 != nil ||
		start.Year() < 1 ||
		end.Year() < 1 ||
		start.After(end) {
		return ReportSummary{}, fmt.Errorf(
			"%w: нужны from <= to в формате YYYY-MM-DD",
			ErrInvalidPeriod,
		)
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		operationTimeout,
	)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return ReportSummary{}, err
	}

	key := "report:summary:" + from + ":" + to

	if cache != nil {
		data, err := cache.Get(ctx, key).Bytes()

		var result ReportSummary

		if err == nil &&
			json.Unmarshal(data, &result) == nil &&
			result.From == from &&
			result.To == to &&
			result.Items != nil {
			log.Printf("report cache hit: %s", key)
			return result, nil
		}

		if err != nil && !errors.Is(err, redis.Nil) {
			log.Printf("report cache read: %v", err)
		}
	}

	log.Printf("report cache miss: %s", key)

	conn, err := database()
	if err != nil {
		return ReportSummary{}, err
	}

	rows, err := conn.QueryContext(
		ctx,
		"SELECT category, (SUM(amount)*100)::bigint FROM expenses WHERE date BETWEEN $1::date AND $2::date GROUP BY category ORDER BY category",
		from,
		to,
	)
	if err != nil {
		return ReportSummary{}, err
	}
	defer rows.Close()

	result := ReportSummary{
		From:  from,
		To:    to,
		Items: make([]ReportItem, 0),
	}

	for rows.Next() {
		var item ReportItem

		if err := rows.Scan(
			&item.Category,
			&item.Amount,
		); err != nil {
			return ReportSummary{}, err
		}

		if (item.Amount > 0 &&
			result.Total > math.MaxInt64-item.Amount) ||
			(item.Amount < 0 &&
				result.Total < math.MinInt64-item.Amount) {
			return ReportSummary{}, errors.New(
				"сумма отчёта выходит за диапазон int64",
			)
		}

		result.Total += item.Amount
		result.Items = append(result.Items, item)
	}

	if err := rows.Err(); err != nil {
		return ReportSummary{}, err
	}

	if cache != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return ReportSummary{}, err
		}

		if err := cache.Set(
			ctx,
			key,
			data,
			reportTTL,
		).Err(); err != nil {
			log.Printf("report cache write: %v", err)
		}
	}

	return result, nil
}
