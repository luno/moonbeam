package address

import (
	"testing"
)

const (
	testBitcoinAddr = "mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2"
	testDomain      = "example.com"
	testAddr        = "mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com"
)

func TestEncode(t *testing.T) {
	actual, err := Encode(testBitcoinAddr, testDomain)
	if err != nil {
		t.Fatal(err)
	}
	if actual != testAddr {
		t.Errorf("Unexpected result: %s", actual)
	}
}

func TestEncodeInvalidBitcoinAddress(t *testing.T) {
	_, err := Encode("mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ3", testDomain)
	if err == nil {
		t.Errorf("Expected error due to invalid bitcoin address")
	}
}

func TestEncodeInvalidDomain(t *testing.T) {
	_, err := Encode(testBitcoinAddr, "ex@mple.com")
	if err == nil {
		t.Errorf("Expected error due to invalid domain")
	}
}

func TestDecode(t *testing.T) {
	bitcoinAddr, domain, valid := Decode(testAddr)
	if !valid {
		t.Errorf("Expected valid")
	}
	if bitcoinAddr != testBitcoinAddr {
		t.Errorf("Unexpected bitcoinAddr: %s", bitcoinAddr)
	}
	if domain != testDomain {
		t.Errorf("Unexpected domain: %s", domain)
	}
}

func TestDecodeInvalidTypo(t *testing.T) {
	// Typo in domain (=> incorrect checksum)
	bitcoinAddr, domain, valid := Decode("mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@examp1e.com")
	if valid {
		t.Errorf("Expected invalid")
	}
	if bitcoinAddr != "" || domain != "" {
		t.Errorf("Expected empty components")
	}
}

func TestDecodeInvalidBitcoinAddress(t *testing.T) {
	// Typo in bitcoinAddress but with correct checksum.
	bitcoinAddr, domain, valid := Decode("mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ3+mb7Jyf9@example.com")
	if valid {
		t.Errorf("Expected invalid")
	}
	if bitcoinAddr != "" || domain != "" {
		t.Errorf("Expected empty components")
	}
}
