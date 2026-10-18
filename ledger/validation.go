package ledger

import "errors"

type Validatable interface {
	Validate() error
}

var _ Validatable = Transaction{}
var _ Validatable = Budget{}

func CheckValid(v Validatable) error {
	if v == nil {
		return errors.New("объект для проверки не задан")
	}

	return v.Validate()
}
