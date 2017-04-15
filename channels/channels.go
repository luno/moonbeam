package channels

import (
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

type Status int

const (
	StatusCreated = 1
	StatusOpen    = 2
	StatusClosing = 3
	StatusClosed  = 4
)

func (s Status) String() string {
	switch s {
	case StatusCreated:
		return "CREATED"
	case StatusOpen:
		return "OPEN"
	case StatusClosing:
		return "CLOSING"
	case StatusClosed:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

const (
	Version = 1
)

var ErrAmountTooSmall = errors.New("amount is too small")
var ErrInsufficientCapacity = errors.New("amount exceeds channel capacity")

func (ss *SharedState) validateAmount(amount int64) (int64, error) {
	if amount <= 0 {
		return ss.Balance, ErrAmountTooSmall
	}
	if amount > ss.Capacity {
		return ss.Balance, ErrInsufficientCapacity
	}

	newBalance := ss.Balance + amount

	if newBalance < dustThreshold {
		return ss.Balance, ErrAmountTooSmall
	}

	if newBalance+ss.Fee > ss.Capacity {
		return ss.Balance, ErrInsufficientCapacity
	}

	return newBalance, nil
}

var ErrInvalidAddress = errors.New("invalid address")

func checkSupportedAddress(net *chaincfg.Params, addr string) error {
	a, err := btcutil.DecodeAddress(addr, net)
	if err != nil {
		return ErrInvalidAddress
	}

	if !a.IsForNet(net) {
		return ErrInvalidAddress
	}

	if _, ok := a.(*btcutil.AddressPubKeyHash); ok {
		return nil
	}
	if _, ok := a.(*btcutil.AddressScriptHash); ok {
		return nil
	}

	return ErrInvalidAddress
}

func derivePubKey(privKey *btcec.PrivateKey, net *chaincfg.Params) (*btcutil.AddressPubKey, error) {
	pk := (*btcec.PublicKey)(&privKey.PublicKey)
	return btcutil.NewAddressPubKey(pk.SerializeCompressed(), net)
}

var ErrNotStatusCreated = errors.New("channel is not in state created")
var ErrNotStatusOpen = errors.New("channel is not in state open")
var ErrNotStatusClosing = errors.New("channel is not in state closing")
