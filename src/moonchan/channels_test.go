package channels

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

func TestFlow(t *testing.T) {
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
		txid   = "be5760e87218c5d613d113b43c2d3760f1edb4a5ca6b35899f33215afa0a7ce4"
		vout   = 0
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

	s.CloseMined()
	s.CloseMined()
}
