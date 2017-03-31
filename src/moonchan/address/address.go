// Package address contains utilities for handling moonbeam addresses.
package address

import (
	"errors"
	"strings"

	"github.com/btcsuite/btcutil/base58"
)

// Encode a moonbeam address for the given bitcoin address and domain.
func Encode(bitcoinAddr, domain string) (string, error) {
	if _, _, err := base58.CheckDecode(bitcoinAddr); err != nil {
		return "", err
	}
	if strings.Contains(domain, "@") {
		return "", errors.New("invalid domain")
	}

	s := bitcoinAddr + "+mb@" + domain

	encoded := base58.CheckEncode([]byte(s), 1)

	version := string(encoded[0])
	checksum := string(encoded[len(encoded)-4:])

	return bitcoinAddr + "+mb" + version + checksum + "@" + domain, nil
}

// Decode a moonbeam address into its constituent bitcoin address and domain.
func Decode(addr string) (bitcoinAddr, domain string, valid bool) {
	i := strings.Index(addr, "@")
	if i < 0 {
		return "", "", false
	}

	before := addr[:i]
	domain = addr[i+1:]

	i = strings.Index(before, "+")
	if i < 0 {
		return "", "", false
	}

	bitcoinAddr = before[:i]

	expected, err := Encode(bitcoinAddr, domain)
	if err != nil {
		return "", "", false
	}

	if addr != expected {
		return "", "", false
	}

	return bitcoinAddr, domain, true
}
