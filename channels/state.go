package channels

import (
	"crypto/sha256"
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

type SharedState struct {
	Version int
	Net     string
	Timeout int64
	Fee     int64

	Status Status

	SenderPubKey   []byte
	ReceiverPubKey []byte

	SenderOutput   string
	ReceiverOutput string

	FundingTxID string
	FundingVout uint32
	Capacity    int64
	BlockHeight int

	Balance      int64
	Count        int
	PaymentsHash [32]byte
	SenderSig    []byte
}

func (ss *SharedState) GetNet() (*chaincfg.Params, error) {
	if ss.Net == NetMain {
		return &chaincfg.MainNetParams, nil
	} else if ss.Net == NetTestnet3 {
		return &chaincfg.TestNet3Params, nil
	} else {
		return nil, errors.New("invalid net")
	}
}

func (ss *SharedState) SenderAddressPubKey() (*btcutil.AddressPubKey, error) {
	net, err := ss.GetNet()
	if err != nil {
		return nil, err
	}
	return btcutil.NewAddressPubKey(ss.SenderPubKey, net)
}

func (ss *SharedState) ReceiverAddressPubKey() (*btcutil.AddressPubKey, error) {
	net, err := ss.GetNet()
	if err != nil {
		return nil, err
	}
	return btcutil.NewAddressPubKey(ss.ReceiverPubKey, net)
}

const (
	NetMain     = "mainnet"
	NetTestnet3 = "testnet3"
)

func netName(net *chaincfg.Params) string {
	if net == &chaincfg.MainNetParams {
		return NetMain
	} else if net == &chaincfg.TestNet3Params {
		return NetTestnet3
	} else {
		return ""
	}
}

const (
	minPaymentSize = 0
	maxPaymentSize = 1 << 16
)

func validatePaymentSize(size int) bool {
	return size > minPaymentSize && size < maxPaymentSize
}

func chainHash(prevHash [32]byte, payment []byte) [32]byte {
	return sha256.Sum256(append(payment, prevHash[:]...))
}
