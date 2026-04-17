package domain

import "testing"

func TestStratumURL_TCP(t *testing.T) {
	u, err := ParseStratumURL("stratum+tcp://pool.example.com:3333")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme() != "stratum+tcp" || u.Host() != "pool.example.com" || u.Port() != "3333" {
		t.Fatalf("got scheme=%q host=%q port=%q", u.Scheme(), u.Host(), u.Port())
	}
	if u.String() != "stratum+tcp://pool.example.com:3333" {
		t.Fatalf("got %q", u.String())
	}
}

func TestStratumURL_SSL(t *testing.T) {
	u, err := ParseStratumURL("stratum+ssl://secure.pool.io:8443")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme() != "stratum+ssl" || u.Host() != "secure.pool.io" || u.Port() != "8443" {
		t.Fatal("unexpected fields")
	}
}

func TestStratumURL_InvalidSchemes(t *testing.T) {
	for _, raw := range []string{
		"http://pool.example.com:3333",
		"tcp://pool.example.com:3333",
	} {
		if _, err := ParseStratumURL(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestStratumURL_MissingHostOrPort(t *testing.T) {
	if _, err := ParseStratumURL("stratum+tcp://:3333"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseStratumURL("stratum+tcp://pool.example.com"); err == nil {
		t.Fatal("expected error")
	}
}

func TestStratumURL_EqualityTrailingSlash(t *testing.T) {
	a, _ := ParseStratumURL("stratum+tcp://167.172.107.33:23334")
	b, _ := ParseStratumURL("stratum+tcp://167.172.107.33:23334/")
	as, ah, ap := a.Key()
	bs, bh, bp := b.Key()
	if as != bs || ah != bh || ap != bp {
		t.Fatal("expected equal keys")
	}
	if a.String() != b.String() {
		t.Fatalf("canonical mismatch: %q vs %q", a.String(), b.String())
	}
}

func TestStratumURL_Inequality(t *testing.T) {
	a, _ := ParseStratumURL("stratum+tcp://pool.example.com:3333")
	b, _ := ParseStratumURL("stratum+tcp://pool.example.com:4444")
	as, ah, ap := a.Key()
	bs, bh, bp := b.Key()
	if as == bs && ah == bh && ap == bp {
		t.Fatal("expected different")
	}
}
