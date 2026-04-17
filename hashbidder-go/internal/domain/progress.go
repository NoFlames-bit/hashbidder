package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type Progress struct {
	value decimal.Decimal
}

func NewProgress(value decimal.Decimal) (Progress, error) {
	if value.IsNegative() || value.GreaterThan(decimal.NewFromInt(1)) {
		return Progress{}, fmt.Errorf("progress must be between 0 and 1, got %s", value)
	}
	return Progress{value: value}, nil
}

func ProgressFromPercentage(pct decimal.Decimal) (Progress, error) {
	return NewProgress(pct.Div(decimal.NewFromInt(100)))
}

func (p Progress) Value() decimal.Decimal      { return p.value }
func (p Progress) Percentage() decimal.Decimal { return p.value.Mul(decimal.NewFromInt(100)) }

func (p Progress) String() string {
	return p.Percentage().String() + "%"
}
