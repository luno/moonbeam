package channels

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

const addr1 = "mrreYyaosje7fxCLi3pzknasHiSfziX9GY"
const addr2 = "mnRYb3Zpn6CUR9TNDL6GGGNY9jjU1XURD5"

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

func TestImmediateClose(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey, addr1)
	if err != nil {
		t.Fatal(err)
	}

	ss := s.State
	ss.ReceiverOutput = addr2
	r, err := AcceptChannel(ss, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ReceivedPubKey(r.State.ReceiverPubKey, addr2, ss.Timeout, ss.Fee)
	if err != nil {
		t.Fatal(err)
	}

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("funding address: %s", addr)

	const (
		txid   = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout   = 1
		amount = 1000000
		height = 100
	)
	sig, err := s.FundingTxMined(txid, vout, amount, height)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Open(txid, vout, amount, height, sig); err != nil {
		t.Fatal(err)
	}

	closeTx, err := r.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("closeTx: %s", hex.EncodeToString(closeTx))

	if err := s.State.validateTx(closeTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}

	if err := s.CloseReceived(closeTx); err != nil {
		t.Fatal(err)
	}

	s.CloseMined()
	r.CloseMined()
}

func TestRefund(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey, addr1)
	if err != nil {
		t.Fatal(err)
	}

	ss := s.State
	ss.ReceiverOutput = addr2
	r, err := AcceptChannel(ss, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ReceivedPubKey(r.State.ReceiverPubKey, addr2, ss.Timeout, ss.Fee)
	if err != nil {
		t.Fatal(err)
	}

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("funding address: %s", addr)

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
	t.Logf("refundTx: %s", hex.EncodeToString(refundTx))

	if err := s.State.validateTx(refundTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}
	if err := r.State.validateTx(refundTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}
}

func TestSend(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey, addr1)
	if err != nil {
		t.Fatal(err)
	}

	ss := s.State
	ss.ReceiverOutput = addr2
	r, err := AcceptChannel(ss, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ReceivedPubKey(r.State.ReceiverPubKey, addr2, ss.Timeout, ss.Fee)
	if err != nil {
		t.Fatal(err)
	}

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("funding address: %s", addr)

	const (
		txid       = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout       = 1
		fundAmount = 1000000
		height     = 100
	)
	sig, err := s.FundingTxMined(txid, vout, fundAmount, height)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Open(txid, vout, fundAmount, height, sig); err != nil {
		t.Fatal(err)
	}

	const amount = 1000

	sig, err = s.PrepareSend(amount)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Send(amount, sig); err != nil {
		t.Fatal(err)
	}
	if err := s.SendAccepted(amount); err != nil {
		t.Fatal(err)
	}

	sig, err = s.PrepareSend(amount * 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Send(amount*2, sig); err != nil {
		t.Fatal(err)
	}
	if err := s.SendAccepted(amount * 2); err != nil {
		t.Fatal(err)
	}

	closeTx, err := r.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("closeTx: %s", hex.EncodeToString(closeTx))

	if err := s.State.validateTx(closeTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}

	if err := s.CloseReceived(closeTx); err != nil {
		t.Fatal(err)
	}

	s.CloseMined()
	r.CloseMined()
}

func TestInvalidSendSig(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey, addr1)
	if err != nil {
		t.Fatal(err)
	}

	ss := s.State
	ss.ReceiverOutput = addr2
	r, err := AcceptChannel(ss, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ReceivedPubKey(r.State.ReceiverPubKey, addr2, ss.Timeout, ss.Fee)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.State.GetFundingScript(); err != nil {
		t.Fatal(err)
	}

	const (
		txid       = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout       = 1
		fundAmount = 1000000
		height     = 100
	)
	sig, err := s.FundingTxMined(txid, vout, fundAmount, height)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Open(txid, vout, fundAmount, height, sig); err != nil {
		t.Fatal(err)
	}

	const amount = 1000

	if err := r.Send(amount, nil); err == nil {
		t.Errorf("Expected error due invalid signature")
	}

	sig, err = s.PrepareSend(amount * 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Send(amount, sig); err == nil {
		t.Errorf("Expected error due invalid signature")
	}
}

func TestSendDust(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey, addr1)
	if err != nil {
		t.Fatal(err)
	}

	ss := s.State
	ss.ReceiverOutput = addr2
	r, err := AcceptChannel(ss, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ReceivedPubKey(r.State.ReceiverPubKey, addr2, ss.Timeout, ss.Fee)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.State.GetFundingScript(); err != nil {
		t.Fatal(err)
	}

	const (
		txid       = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout       = 1
		fundAmount = 1000000
		height     = 100
	)
	sig, err := s.FundingTxMined(txid, vout, fundAmount, height)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Open(txid, vout, fundAmount, height, sig); err != nil {
		t.Fatal(err)
	}

	const amount = 100

	sig, err = s.PrepareSend(amount)
	if err == nil {
		t.Errorf("Expected error due to amount too small")
	}

	sig, err = s.signBalance(amount)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Send(amount, sig); err == nil {
		t.Errorf("Expected error due to dust output")
	}
}

func TestCapacityTooLow(t *testing.T) {
	net, senderWIF, receiverWIF := setUp(t)

	s, err := OpenChannel(net, senderWIF.PrivKey, addr1)
	if err != nil {
		t.Fatal(err)
	}

	ss := s.State
	ss.ReceiverOutput = addr2
	r, err := AcceptChannel(ss, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ReceivedPubKey(r.State.ReceiverPubKey, addr2, ss.Timeout, ss.Fee)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.State.GetFundingScript(); err != nil {
		t.Fatal(err)
	}

	fundAmount := s.State.Fee + dustThreshold - 1

	const (
		txid   = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout   = 1
		height = 100
	)
	sig, err := s.FundingTxMined(txid, vout, fundAmount, height)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Open(txid, vout, fundAmount, height, sig); err == nil {
		t.Errorf("Expected error due to capacity too low")
	}
	if r.State.Status != StatusCreated {
		t.Errorf("Wrong status: %s", r.State.Status)
	}
}

func TestValidateAmount(t *testing.T) {
	var s SharedState
	s.Balance = 1000
	s.Capacity = 100000
	s.Fee = 100

	if nb, err := s.validateAmount(100); nb != 1100 || err != nil {
		t.Errorf("Unexpected result: %d", nb)
	}

	if nb, err := s.validateAmount(98900); nb != 99900 || err != nil {
		t.Errorf("Unexpected result: %d", nb)
	}

	if _, err := s.validateAmount(0); err != ErrAmountTooSmall {
		t.Errorf("Expected ErrAmountTooSmall, got: %v", err)
	}

	if _, err := s.validateAmount(-100); err != ErrAmountTooSmall {
		t.Errorf("Expected ErrAmountTooSmall, got: %v", err)
	}

	if _, err := s.validateAmount(98901); err != ErrInsufficientCapacity {
		t.Errorf("Expected ErrInsufficientCapacity, got: %v", err)
	}

	if _, err := s.validateAmount(s.Capacity); err != ErrInsufficientCapacity {
		t.Errorf("Expected ErrInsufficientCapacity, got: %v", err)
	}

	// Overflow
	if _, err := s.validateAmount(1<<63 - 100); err != ErrInsufficientCapacity {
		t.Errorf("Expected ErrInsufficientCapacity, got: %v", err)
	}
}
