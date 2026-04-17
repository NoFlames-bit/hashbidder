package domain

import (
	"fmt"
	"sort"
)

type HashUnit int64

const (
	H  HashUnit = 1
	KH HashUnit = 1_000
	MH HashUnit = 1_000_000
	GH HashUnit = 1_000_000_000
	TH HashUnit = 1_000_000_000_000
	PH HashUnit = 1_000_000_000_000_000
	EH HashUnit = 1_000_000_000_000_000_000
)

var hashUnitOrder = []HashUnit{H, KH, MH, GH, TH, PH, EH}

func (u HashUnit) Name() string {
	switch u {
	case H:
		return "H"
	case KH:
		return "KH"
	case MH:
		return "MH"
	case GH:
		return "GH"
	case TH:
		return "TH"
	case PH:
		return "PH"
	case EH:
		return "EH"
	default:
		return "H"
	}
}

func HashUnitFromRateStr(s string) (HashUnit, error) {
	m := map[string]HashUnit{
		"H/s":  H,
		"h/s":  H,
		"KH/s": KH,
		"Kh/s": KH,
		"MH/s": MH,
		"Mh/s": MH,
		"GH/s": GH,
		"Gh/s": GH,
		"TH/s": TH,
		"Th/s": TH,
		"PH/s": PH,
		"Ph/s": PH,
		"EH/s": EH,
		"Eh/s": EH,
	}
	u, ok := m[s]
	if !ok {
		return 0, fmt.Errorf("unrecognized hashrate unit: %q", s)
	}
	return u, nil
}

func sortedUnitsAsc() []HashUnit {
	out := make([]HashUnit, len(hashUnitOrder))
	copy(out, hashUnitOrder)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
