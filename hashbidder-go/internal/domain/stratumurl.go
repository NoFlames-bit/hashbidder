package domain

import (
	"fmt"
	"net/url"
	"strings"
)

var validSchemes = map[string]struct{}{
	"stratum+tcp": {},
	"stratum+ssl": {},
}

type StratumURL struct {
	raw    string
	scheme string
	host   string
	port   string
}

func ParseStratumURL(raw string) (StratumURL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return StratumURL{}, err
	}
	scheme := strings.ToLower(u.Scheme)
	if _, ok := validSchemes[scheme]; !ok {
		return StratumURL{}, fmt.Errorf("invalid stratum URL scheme %q, expected one of [stratum+tcp stratum+ssl]", u.Scheme)
	}
	if u.Hostname() == "" {
		return StratumURL{}, fmt.Errorf("stratum URL must have a host: %q", raw)
	}
	port := u.Port()
	if port == "" {
		return StratumURL{}, fmt.Errorf("stratum URL must have a port: %q", raw)
	}
	host := u.Hostname()
	// Canonical form so trailing slashes / stray paths do not affect equality or String().
	canon := fmt.Sprintf("%s://%s:%s", scheme, host, port)
	return StratumURL{raw: canon, scheme: scheme, host: host, port: port}, nil
}

func (s StratumURL) Scheme() string { return s.scheme }
func (s StratumURL) Host() string   { return s.host }
func (s StratumURL) Port() string   { return s.port }

func (s StratumURL) String() string {
	return fmt.Sprintf("%s://%s:%s", s.scheme, s.host, s.port)
}

func (s StratumURL) Key() (string, string, string) {
	return s.scheme, s.host, s.port
}
