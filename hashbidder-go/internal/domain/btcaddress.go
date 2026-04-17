package domain

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58Map = func() map[rune]int {
	m := make(map[rune]int)
	for i, c := range base58Alphabet {
		m[c] = i
	}
	return m
}()

var base58Re = regexp.MustCompile("^[" + base58Alphabet + "]+$")

func base58Decode(s string) ([]byte, error) {
	if !base58Re.MatchString(s) {
		return nil, fmt.Errorf("invalid base58 characters")
	}
	n := new(big.Int)
	rad := big.NewInt(58)
	for _, c := range s {
		v, ok := base58Map[c]
		if !ok {
			return nil, fmt.Errorf("invalid base58")
		}
		n.Mul(n, rad)
		n.Add(n, big.NewInt(int64(v)))
	}
	leading := len(s) - len(strings.TrimLeft(s, "1"))
	raw := n.Bytes()
	if len(raw) == 0 {
		raw = []byte{0}
	}
	out := append(bytes.Repeat([]byte{0}, leading), raw...)
	return out, nil
}

func validateBase58check(value string) error {
	if len(value) < 25 || len(value) > 34 {
		return fmt.Errorf("base58 address must be 25-34 characters, got %d", len(value))
	}
	decoded, err := base58Decode(value)
	if err != nil {
		return err
	}
	if len(decoded) != 25 {
		return errors.New("base58 address decodes to wrong length")
	}
	payload := decoded[:21]
	checksum := decoded[21:]
	h := sha256.Sum256(payload)
	h2 := sha256.Sum256(h[:])
	if !bytes.Equal(checksum, h2[:4]) {
		return errors.New("base58check checksum mismatch")
	}
	return nil
}

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var bech32Map = func() map[rune]int {
	m := make(map[rune]int)
	for i, c := range bech32Charset {
		m[c] = i
	}
	return m
}()

var bech32Re = regexp.MustCompile(`^bc1[ac-hj-np-z02-9]+$`)

const bech32Const = 1
const bech32mConst = 0x2BC830A3

func bech32Polymod(values []int) int {
	gen := []int{0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3}
	chk := 1
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1FFFFFF)<<5 ^ v
		for i := 0; i < 5; i++ {
			if (top>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32HrpExpand(hrp string) []int {
	out := make([]int, 0, len(hrp)*2+1)
	for _, x := range hrp {
		out = append(out, int(x>>5))
	}
	out = append(out, 0)
	for _, x := range hrp {
		out = append(out, int(x&31))
	}
	return out
}

func validateBech32(value string) error {
	lower := strings.ToLower(value)
	if lower != value {
		return fmt.Errorf("bech32 address must be lowercase: %q", value)
	}
	if !bech32Re.MatchString(lower) {
		return fmt.Errorf("invalid bech32 characters: %q", value)
	}
	if len(value) != 42 && len(value) != 62 {
		return fmt.Errorf("bech32 address must be 42 or 62 characters, got %d", len(value))
	}
	dataPart := lower[3:]
	data := make([]int, 0, len(dataPart))
	for _, c := range dataPart {
		v, ok := bech32Map[c]
		if !ok {
			return fmt.Errorf("invalid bech32 data char: %q", c)
		}
		data = append(data, v)
	}
	witnessVersion := data[0]
	expected := bech32Const
	if witnessVersion != 0 {
		expected = bech32mConst
	}
	if bech32Polymod(append(bech32HrpExpand("bc"), data...)) != expected {
		return errors.New("bech32 checksum mismatch")
	}
	return nil
}

func validateBTCAddress(value string) error {
	if value == "" {
		return errors.New("BTC address must not be empty")
	}
	if strings.HasPrefix(strings.ToLower(value), "bc1") {
		return validateBech32(value)
	}
	if strings.HasPrefix(value, "1") || strings.HasPrefix(value, "3") {
		return validateBase58check(value)
	}
	return fmt.Errorf("unrecognized address format (expected prefix 1, 3, or bc1): %q", value)
}

type BtcAddress struct {
	value string
}

func ParseBtcAddress(raw string) (BtcAddress, error) {
	stripped := strings.TrimSpace(raw)
	if err := validateBTCAddress(stripped); err != nil {
		return BtcAddress{}, err
	}
	return BtcAddress{value: stripped}, nil
}

func (a BtcAddress) Value() string { return a.value }

func (a BtcAddress) Truncated() string {
	if len(a.value) <= 14 {
		return a.value
	}
	return a.value[:7] + "..." + a.value[len(a.value)-4:]
}

func (a BtcAddress) String() string { return a.value }
