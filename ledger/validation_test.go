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

func TestCheckValidDelegatesAndDoesNotStore(t *testing.T) {
	resetTransactions(t)

	cause := errors.New("validation failed")
	probe := &validationProbe{err: cause}

	if err := CheckValid(probe); err != cause || !probe.called {
		t.Fatalf("Validate was not delegated correctly: %v", err)
	}

	if err := CheckValid(nil); err == nil {
		t.Fatal("nil must fail")
	}

	for _, value := range []Validatable{
		testTransaction(100, "еда"),
		Budget{Category: "еда", Limit: 100},
	} {
		if err := CheckValid(value); err != nil {
			t.Fatal(err)
		}
	}

	if len(transactions) != 0 || len(budgets) != 0 {
		t.Fatal("validation must not save objects")
	}
}
