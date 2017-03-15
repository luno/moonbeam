package channels

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

func setUp(t *testing.T) (*chaincfg.Params, *btcutil.WIF, *btcutil.WIF) {
	net := &chaincfg.TestNet3Params

	const senderPrivKeyWIF = "cRTgZtoTP8ueH4w7nob5reYTKpFLHvDV9UfUfa67f3SMCaZkGB6L"
	const receiverPrivKeyWIF = "cUkJhR6V9Gjrw1enLJ7AHk37Bhtmfk3AyWkRLVhvHGYXSPj3mDLq"
	senderWIF, err := btcutil.DecodeWIF(senderPrivKeyWIF)
	if err != nil {
		t.Fatal(err)
	}
	receiverWIF, err := btcutil.DecodeWIF(receiverPrivKeyWIF)
	if err != nil {
		t.Fatal(err)
	}

	return net, senderWIF, receiverWIF
}

func TestFlow(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	r, err := AcceptChannel(s.State, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	s.ReceivedPubKey(r.State.ReceiverPubKey)

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		t.Fatal(err)
	}
	t.Errorf("funding address: %s", addr)

	const (
		txid   = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout   = 1
		amount = 1000000
		height = 100
	)
	r.FundingTxMined(txid, vout, amount, height)
	s.FundingTxMined(txid, vout, amount, height)

	sig, err := s.CloseBegin()
	if err != nil {
		t.Fatal(err)
	}

	closeTx, err := r.CloseReceived(sig)
	if err != nil {
		t.Fatal(err)
	}
	t.Errorf("closeTx: %s", hex.EncodeToString(closeTx))

	if err := s.State.validateTx(closeTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}

	s.CloseMined()
	s.CloseMined()
}

func TestRefund(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	s.State.Timeout = 1

	r, err := AcceptChannel(s.State, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	s.ReceivedPubKey(r.State.ReceiverPubKey)

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		t.Fatal(err)
	}
	t.Errorf("funding address: %s", addr)

	const (
		txid   = "f4c7b41725dbc9111293a82cae6299aa9e9bf93bc8d46676d4f3a48923329c86"
		vout   = 0
		amount = 1000000
		height = 100
	)
	s.FundingTxMined(txid, vout, amount, height)

	refundTx, err := s.Refund()
	if err != nil {
		t.Fatal(err)
	}
	t.Errorf("refundTx: %s", hex.EncodeToString(refundTx))

	if err := s.State.validateTx(refundTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}
}
