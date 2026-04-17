package domain

import "testing"

func TestBlockSubsidy_Genesis(t *testing.T) {
	h, _ := NewBlockHeight(0)
	if BlockSubsidy(h) != Sats(5_000_000_000) {
		t.Fatalf("got %d", BlockSubsidy(h))
	}
}

func TestBlockSubsidy_LastBeforeFirstHalving(t *testing.T) {
	h, _ := NewBlockHeight(209_999)
	if BlockSubsidy(h) != Sats(5_000_000_000) {
		t.Fatalf("got %d", BlockSubsidy(h))
	}
}

func TestBlockSubsidy_SecondHalving(t *testing.T) {
	h, _ := NewBlockHeight(420_000)
	if BlockSubsidy(h) != Sats(1_250_000_000) {
		t.Fatalf("got %d", BlockSubsidy(h))
	}
}

func TestBlockSubsidy_FarFutureZero(t *testing.T) {
	h, _ := NewBlockHeight(210_000 * 64)
	if BlockSubsidy(h) != 0 {
		t.Fatalf("got %d", BlockSubsidy(h))
	}
}
