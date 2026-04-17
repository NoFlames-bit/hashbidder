package domain

import "testing"

func TestBtcAddress_Valid(t *testing.T) {
	cases := []string{
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"bc1qwzrryqr3ja8w7hnja2spmkgfdcgvqwp5swz4af4ngsjecfz0w0pqud7k38",
		"bc1pgy84xdguk0e6jzaazrn3kfxvtf6mnerxvdq9uyrwejqan48l6u3qdhkz53",
	}
	for _, c := range cases {
		if _, err := ParseBtcAddress(c); err != nil {
			t.Fatalf("%q: %v", c, err)
		}
	}
	a, err := ParseBtcAddress("  1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa  ")
	if err != nil || a.Value() != "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" {
		t.Fatalf("whitespace: %+v err=%v", a, err)
	}
}

func TestBtcAddress_Invalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
		"BC1QW508D6QEJXTDG4Y5R3ZARVARY0C5XW7KV8F3T4",
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7k",
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3b4",
	}
	for _, c := range cases {
		if _, err := ParseBtcAddress(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}
