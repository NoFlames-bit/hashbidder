package domain

import "testing"

func TestBlockHeight_Genesis(t *testing.T) {
	h, err := NewBlockHeight(0)
	if err != nil || h.Value != 0 {
		t.Fatalf("expected 0, got %v err=%v", h, err)
	}
}

func TestBlockHeight_Positive(t *testing.T) {
	h, err := NewBlockHeight(840_000)
	if err != nil || h.Value != 840_000 {
		t.Fatalf("expected 840000, got %v err=%v", h, err)
	}
}

func TestBlockHeight_Negative(t *testing.T) {
	_, err := NewBlockHeight(-1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlockHeight_Equality(t *testing.T) {
	a, _ := NewBlockHeight(100)
	b, _ := NewBlockHeight(100)
	if a != b {
		t.Fatal("expected equal")
	}
	c, _ := NewBlockHeight(200)
	if a == c {
		t.Fatal("expected not equal")
	}
}

func TestBlockHeight_String(t *testing.T) {
	h, _ := NewBlockHeight(42)
	if h.String() != "42" {
		t.Fatalf("got %q", h.String())
	}
}
