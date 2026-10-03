package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Budget — фиксированный бюджет категории за всё время работы процесса.
// Limit, как и Transaction.Amount, задаётся в копейках
type Budget struct {
	Category string `json:"category"`
	Limit    int    `json:"limit"`
}

var budgets = make(map[string]Budget)

// ErrBudgetExceeded позволяет распознать превышение через errors.Is
var ErrBudgetExceeded = errors.New("budget exceeded")

// SetBudget добавляет или заменяет бюджет; историю транзакций не изменяет
// LoadBudgets проверяет эти условия для внешних JSON-данных
func SetBudget(b Budget) {
	if budgets == nil {
		budgets = make(map[string]Budget)
	}
	budgets[b.Category] = b
}

// LoadBudgets читает один JSON-массив. При любой ошибке бюджеты не изменяются
// Если категория повторяется, последнее значение заменяет предыдущие
func LoadBudgets(r io.Reader) error {
	if r == nil {
		return errors.New("загрузка бюджетов: источник чтения не задан")
	}

	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	// Указатель отличает отсутствующий/null limit от допустимого нулевого лимита.
	var input []struct {
		Category string `json:"category"`
		Limit    *int   `json:"limit"`
	}

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

	// Сначала проверяем весь массив, чтобы избежать частичной загрузки.
	for i, item := range input {
		if strings.TrimSpace(item.Category) == "" {
			return fmt.Errorf(
				"загрузка бюджетов: объект %d: category должна быть непустой строкой",
				i+1,
			)
		}

		if item.Limit == nil || *item.Limit < 0 {
			return fmt.Errorf(
				"загрузка бюджетов: объект %d: limit обязателен и должен быть неотрицательным целым числом копеек",
				i+1,
			)
		}
	}

	for _, item := range input {
		SetBudget(Budget{
			Category: item.Category,
			Limit:    *item.Limit,
		})
	}

	return nil
}
