package ledger

import "testing"

func TestTransactionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   int
		category string
		date     string
		wantErr  bool
	}{
		{"valid", 100, "еда", "2026-10-22", false},
		{"zero_amount", 0, "еда", "2026-10-22", true},
		{"negative_amount", -100, "еда", "2026-10-22", true},
		{"empty_category", 100, "", "2026-10-22", true},
		{"blank_category", 100, " \t", "2026-10-22", true},
		{"zero_year", 100, "еда", "0000-01-01", true},
		{"too_large", int(maxMoney + 1), "еда", "2026-10-30", true},
		{"missing_date", 100, "еда", "", true},
		{"wrong_date_format", 100, "еда", "22.10.2026", true},
		{"impossible_date", 100, "еда", "2026-02-30", true},
		{"leap_year", 100, "еда", "2024-02-29", false},
		{"not_leap_year", 100, "еда", "2025-02-29", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tx := Transaction{
				Amount:   tt.amount,
				Category: tt.category,
				Date:     tt.date,
			}

			if err := tx.Validate(); (err != nil) != tt.wantErr {
				t.Errorf(
					"Validate() = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		budget  Budget
		wantErr bool
	}{
		{
			"valid",
			Budget{Category: "еда", Limit: 5000},
			false,
		},
		{
			"too_large",
			Budget{Category: "еда", Limit: int(maxMoney + 1)},
			true,
		},
		{
			"zero_limit",
			Budget{Category: "еда", Limit: 0},
			true,
		},
		{
			"negative_limit",
			Budget{Category: "еда", Limit: -1},
			true,
		},
		{
			"empty_category",
			Budget{Category: "", Limit: 5000},
			true,
		},
		{
			"blank_category",
			Budget{Category: " \t", Limit: 5000},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.budget.Validate(); (err != nil) != tt.wantErr {
				t.Errorf(
					"Validate() = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
