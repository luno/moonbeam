package channels

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

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

	Balance   int64
	Count     int
	SenderSig []byte

	Payments [][]byte
}

func (ss *SharedState) GetNet() (*chaincfg.Params, error) {
	if ss.Net == netMain {
		return &chaincfg.MainNetParams, nil
	} else if ss.Net == netTestnet3 {
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
	netMain     = "main"
	netTestnet3 = "testnet3"
)

func netName(net *chaincfg.Params) string {
	if net == &chaincfg.MainNetParams {
		return netMain
	} else if net == &chaincfg.TestNet3Params {
		return netTestnet3
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
func writePayment(w io.Writer, p []byte) error {
	if err := binary.Write(w, binary.LittleEndian, len(p)); err != nil {
		return err
	}
	_, err := w.Write(p)
	return err
}

func hashPayments(pl [][]byte) ([]byte, error) {
	h := sha256.New()
	for _, p := range pl {
		if err := writePayment(h, p); err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}
