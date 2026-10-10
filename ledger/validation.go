package main

import "errors"

// Validatable задаёт общий контракт проверки данных.
type Validatable interface {
	Validate() error
}

// Проверяем при компиляции, что оба типа реализуют интерфейс.
var _ Validatable = Transaction{}
var _ Validatable = Budget{}

// CheckValid вызывает проверку объекта через интерфейс.
func CheckValid(v Validatable) error {
	if v == nil {
		return errors.New("объект для проверки не задан")
	}

	return v.Validate()
}
