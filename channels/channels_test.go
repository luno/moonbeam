package channels

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"bitbucket.org/bitx/moonchan/models"
)

const addr1 = "mrreYyaosje7fxCLi3pzknasHiSfziX9GY"
const addr2 = "mnRYb3Zpn6CUR9TNDL6GGGNY9jjU1XURD5"

var testPayment = []byte{1, 2, 3}

const testCapacity = 1000000

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

func setUpChannel(t *testing.T, capacity int64) (*Sender, *Receiver) {
	_, senderWIF, receiverWIF := setUp(t)

	s, err := NewSender(DefaultSenderConfig, senderWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	createReq, err := s.GetCreateRequest(addr1)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReceiver(DefaultReceiverConfig, addr2, receiverWIF.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	createResp, err := r.Create(createReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GotCreateResponse(createResp); err != nil {
		t.Fatal(err)
	}

	const (
		txid        = "5b2c6c349612986a3e012bbc79e5e04d5ba965f0e8f968cf28c91681acbbeb34"
		vout        = 1
		pkscriptHex = "fbe9351367de8e1e341ad62312f107b839bddb0a"
	)
	pkscript, _ := hex.DecodeString(pkscriptHex)
	txout := wire.NewTxOut(capacity, pkscript)

	openReq, err := s.GetOpenRequest(txid, vout, capacity)
	if err != nil {
		t.Fatal(err)
	}
	openResp, err := r.Open(txout, openReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GotOpenResponse(openResp); err != nil {
		t.Fatal(err)
	}

	if s.State.Status != StatusOpen {
		t.Errorf("expected sender to be in open state")
	}
	if r.State.Status != StatusOpen {
		t.Errorf("expected receiver to be in open state")
	}

	return s, r
}

func closeChannels(t *testing.T, s *Sender, r *Receiver) {
	closeReq, err := s.GetCloseRequest()
	if err != nil {
		t.Fatal(err)
	}
	closeResp, err := r.Close(closeReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GotCloseResponse(closeResp); err != nil {
		t.Fatal(err)
	}

	if s.State.Status != StatusClosing {
		t.Errorf("expected sender to be in closing state")
	}
	if r.State.Status != StatusClosing {
		t.Errorf("expected receiver to be in closing state")
	}
}

func TestImmediateClose(t *testing.T) {
	s, r := setUpChannel(t, testCapacity)
	closeChannels(t, s, r)
}

func TestRefund(t *testing.T) {
	s, r := setUpChannel(t, testCapacity)

	refundTx, err := s.Refund()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.State.validateTx(refundTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}
	if err := r.State.validateTx(refundTx); err != nil {
		t.Errorf("validateTx error: %v", err)
	}
}

func TestSend(t *testing.T) {
	s, r := setUpChannel(t, testCapacity)

	const amount = 1000

	valid, err := r.Validate(amount, testPayment)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Errorf("Expected valid payment, got: %+v", valid)
	}

	sendReq, err := s.GetSendRequest(amount, testPayment)
	if err != nil {
		t.Fatal(err)
	}
	sendResp, err := r.Send(amount, sendReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GotSendResponse(amount, testPayment, sendResp); err != nil {
		t.Fatal(err)
	}
	if r.State.Balance != amount {
		t.Errorf("Unexpected receiver balance: %+v", r.State)
	}
	if s.State.Balance != amount {
		t.Errorf("Unexpected sender balance: %+v", s.State)
	}

	sendReq, err = s.GetSendRequest(2*amount, testPayment)
	if err != nil {
		t.Fatal(err)
	}
	sendResp, err = r.Send(2*amount, sendReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GotSendResponse(2*amount, testPayment, sendResp); err != nil {
		t.Fatal(err)
	}
	if r.State.Balance != 3*amount {
		t.Errorf("Unexpected receiver balance: %+v", r.State)
	}
	if s.State.Balance != 3*amount {
		t.Errorf("Unexpected sender balance: %+v", s.State)
	}

	if s.State.Status != StatusOpen {
		t.Errorf("expected sender to be in open state")
	}
	if r.State.Status != StatusOpen {
		t.Errorf("expected receiver to be in open state")
	}

	closeChannels(t, s, r)
}

func TestInvalidSendSig(t *testing.T) {
	s, r := setUpChannel(t, testCapacity)

	const amount = 1000

	sendReq := &models.SendRequest{
		Payment: testPayment,
	}
	if _, err := r.Send(amount, sendReq); err == nil {
		t.Errorf("Expected error due invalid signature")
	}

	sendReq, err := s.GetSendRequest(amount, testPayment)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Send(amount*2, sendReq); err == nil {
		t.Errorf("Expected error due invalid signature")
	}
}

func TestSendDust(t *testing.T) {
	s, r := setUpChannel(t, testCapacity)

	const amount = 100

	if _, err := s.GetSendRequest(amount, testPayment); err == nil {
		t.Errorf("Expected error due to amount too small")
	}

	newHash := chainHash(s.State.PaymentsHash, testPayment)
	sig, err := s.signBalance(amount, newHash)
	if err != nil {
		t.Fatal(err)
	}
	sendReq := &models.SendRequest{
		Payment:   testPayment,
		SenderSig: sig,
	}
	if _, err := r.Send(amount, sendReq); err == nil {
		t.Errorf("Expected error due to dust output")
	}
}

// If the channel was funded with an amount too small for any payments, we can
// at least still allow the sender to attempt to close it cleanly.
func TestLowCapacity(t *testing.T) {
	s, r := setUpChannel(t, dustThreshold)
	closeChannels(t, s, r)
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
