package channels

import (
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

type SimpleSharedState struct {
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
}

const (
	netMain     = "main"
	netTestnet3 = "testnet3"
)

func (ss *SharedState) ToSimple() (*SimpleSharedState, error) {
	net := ""
	if ss.Net == &chaincfg.MainNetParams {
		net = netMain
	} else if ss.Net == &chaincfg.TestNet3Params {
		net = netTestnet3
	} else {
		return nil, errors.New("invalid net")
	}

	return &SimpleSharedState{
		Version:        ss.Version,
		Net:            net,
		Timeout:        ss.Timeout,
		Fee:            ss.Fee,
		Status:         ss.Status,
		SenderOutput:   ss.SenderOutput,
		ReceiverOutput: ss.ReceiverOutput,
		SenderPubKey:   ss.SenderPubKey.PubKey().SerializeCompressed(),
		ReceiverPubKey: ss.ReceiverPubKey.PubKey().SerializeCompressed(),
		FundingTxID:    ss.FundingTxID,
		FundingVout:    ss.FundingVout,
		Capacity:       ss.Capacity,
		BlockHeight:    ss.BlockHeight,
		Balance:        ss.Balance,
		Count:          ss.Count,
		SenderSig:      ss.SenderSig,
	}, nil
}

func FromSimple(s SimpleSharedState) (*SharedState, error) {
	var net *chaincfg.Params
	if s.Net == netMain {
		net = &chaincfg.MainNetParams
	} else if s.Net == netTestnet3 {
		net = &chaincfg.TestNet3Params
	} else {
		return nil, errors.New("invalid net")
	}

	senderPubKey, err := btcutil.NewAddressPubKey(s.SenderPubKey, net)
	if err != nil {
		return nil, err
	}
	receiverPubKey, err := btcutil.NewAddressPubKey(s.ReceiverPubKey, net)
	if err != nil {
		return nil, err
	}

	ss := SharedState{
		Version:        s.Version,
		Net:            net,
		Timeout:        s.Timeout,
		Fee:            s.Fee,
		Status:         s.Status,
		SenderOutput:   s.SenderOutput,
		ReceiverOutput: s.ReceiverOutput,
		SenderPubKey:   senderPubKey,
		ReceiverPubKey: receiverPubKey,
		FundingTxID:    s.FundingTxID,
		FundingVout:    s.FundingVout,
		Capacity:       s.Capacity,
		BlockHeight:    s.BlockHeight,
		Balance:        s.Balance,
		Count:          s.Count,
		SenderSig:      s.SenderSig,
	}
	return &ss, nil
}
