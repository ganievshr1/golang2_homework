package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Budget — фиксированный бюджет категории
// Limit задаётся в копейках
type Budget struct {
	Category string `json:"category"`
	Limit    int    `json:"limit"`
}

// Validate проверяет только данные бюджета
// Метод не логирует и не изменяет хранилище
func (b Budget) Validate() error {
	if b.Limit <= 0 {
		return errors.New("лимит бюджета должен быть положительным")
	}

	if strings.TrimSpace(b.Category) == "" {
		return errors.New("категория бюджета не должна быть пустой")
	}

	return nil
}

var budgets = make(map[string]Budget)

var ErrBudgetExceeded = errors.New("budget exceeded")

// SetBudget добавляет или заменяет только корректный бюджет
// При ошибке существующий бюджет остаётся без изменений
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

// LoadBudgets читает один JSON-массив
// Все объекты проверяются до изменения хранилища
func LoadBudgets(r io.Reader) error {
	if r == nil {
		return errors.New("загрузка бюджетов: источник чтения не задан")
	}

	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	// Отсутствующий или null limit даст 0,
	// который будет отклонён методом Validate.
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

	// Сначала проверяем весь массив
	for i, item := range input {
		if err := item.Validate(); err != nil {
			return fmt.Errorf(
				"загрузка бюджетов: объект %d: %w",
				i+1,
				err,
			)
		}
	}

	// После успешной проверки добавляем бюджеты
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
