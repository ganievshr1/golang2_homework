package ledger

import (
	"errors"
	"testing"
)

type validationProbe struct {
	called bool
	err    error
}

func (p *validationProbe) Validate() error {
	p.called = true
	return p.err
}

func TestCheckValid(t *testing.T) {
	cause := errors.New("validation failed")
	p := &validationProbe{err: cause}

	if err := CheckValid(p); err != cause || !p.called {
		t.Fatal("Validate was not delegated")
	}

	if CheckValid(nil) == nil {
		t.Fatal("nil accepted")
	}

	values := []Validatable{
		Transaction{
			Amount:   100,
			Category: "еда",
			Date:     "2026-10-30",
		},
		Budget{
			Category: "еда",
			Limit:    100,
		},
	}

	for _, value := range values {
		if err := CheckValid(value); err != nil {
			t.Fatal(err)
		}
	}
}
