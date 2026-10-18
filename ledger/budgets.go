package ledger

import (
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

func (b Budget) Validate() error {
	if b.Limit <= 0 {
		return errors.New(
			"лимит бюджета должен быть положительным",
		)
	}

	if strings.TrimSpace(b.Category) == "" {
		return errors.New(
			"категория бюджета не должна быть пустой",
		)
	}

	return nil
}

var budgets = make(map[string]Budget)

var ErrBudgetExceeded = errors.New("budget exceeded")

func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return fmt.Errorf("бюджет: %w", err)
	}

	if budgets == nil {
		budgets = make(map[string]Budget)
	}

	budgets[b.Category] = b
	return nil
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
			"загрузка бюджетов: не удалось прочитать JSON-массив: %w",
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
				"загрузка бюджетов: ошибка чтения после JSON-массива: %w",
				err,
			)
		}

		return errors.New(
			"загрузка бюджетов: после массива обнаружено лишнее JSON-значение",
		)
	}

	// Сначала проверяем весь массив.
	for i, item := range input {
		if err := item.Validate(); err != nil {
			return fmt.Errorf(
				"загрузка бюджетов: объект %d: %w",
				i+1,
				err,
			)
		}
	}

	// Записываем только после успешной проверки.
	for i, item := range input {
		if err := SetBudget(item); err != nil {
			return fmt.Errorf(
				"загрузка бюджетов: объект %d: %w",
				i+1,
				err,
			)
		}
	}

	return nil
}

func ListBudgets() []Budget {
	result := make([]Budget, 0, len(budgets))

	for _, b := range budgets {
		result = append(result, b)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Category < result[j].Category
	})

	return result
}
