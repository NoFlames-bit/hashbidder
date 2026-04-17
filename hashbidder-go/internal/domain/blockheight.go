package domain

import "fmt"

type BlockHeight struct {
	Value int
}

func NewBlockHeight(v int) (BlockHeight, error) {
	if v < 0 {
		return BlockHeight{}, fmt.Errorf("block height must be non-negative, got %d", v)
	}
	return BlockHeight{Value: v}, nil
}

func (h BlockHeight) String() string {
	return fmt.Sprintf("%d", h.Value)
}
