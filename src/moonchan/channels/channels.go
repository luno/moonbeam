package channels

import (
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

type Status int

const (
	StatusNotStarted      = 1
	StatusPreInfoGathered = 2
	StatusOpen            = 3
	StatusClosing         = 4
	StatusClosed          = 5
)

const defaultTimeout = 144

type SharedState struct {
	Version int
	Net     *chaincfg.Params
	Timeout int64
	Fee     int64

	Status Status

	SenderPubKey   *btcutil.AddressPubKey
	ReceiverPubKey *btcutil.AddressPubKey

	FundingTxID   string
	FundingVout   uint32
	FundingAmount int64
	BlockHeight   int

	Balance int64
	Count   int
}

func (ss *SharedState) validateAmount(amount int64) (int64, error) {
	if amount <= 0 {
		return ss.Balance, errors.New("amount must be positive")
	}

	newBalance := ss.Balance + amount

	if newBalance+ss.Fee > ss.FundingAmount {
		return ss.Balance, errors.New("insufficient channel capacity")
	}

	return newBalance, nil
}

func DefaultState(net *chaincfg.Params) SharedState {
	return SharedState{
		Version: 1,
		Net:     net,
		Timeout: defaultTimeout,
		Fee:     5000,
		Status:  StatusNotStarted,
	}
}

type Sender struct {
	State   SharedState
	PrivKey *btcec.PrivateKey
}

func NewSender(state SharedState, privKey *btcec.PrivateKey) (*Sender, error) {
	return &Sender{state, privKey}, nil
}

func derivePubKey(privKey *btcec.PrivateKey, net *chaincfg.Params) (*btcutil.AddressPubKey, error) {
	pk := (*btcec.PublicKey)(&privKey.PublicKey)
	return btcutil.NewAddressPubKey(pk.SerializeCompressed(), net)
}

func OpenChannel(net *chaincfg.Params, privKey *btcec.PrivateKey) (*Sender, error) {
	pubKey, err := derivePubKey(privKey, net)
	if err != nil {
		return nil, err
	}

	ss := DefaultState(net)
	ss.SenderPubKey = pubKey

	c := Sender{
		State:   ss,
		PrivKey: privKey,
	}
	return &c, nil
}

func (s *Sender) ReceivedPubKey(pubKey *btcutil.AddressPubKey) {
	s.State.ReceiverPubKey = pubKey
}

type Receiver struct {
	State     SharedState
	PrivKey   *btcec.PrivateKey
	SenderSig []byte
}

func AcceptChannel(state SharedState, privKey *btcec.PrivateKey) (*Receiver, error) {
	pubKey, err := derivePubKey(privKey, state.Net)
	if err != nil {
		return nil, err
	}

	state.ReceiverPubKey = pubKey
	state.Status = StatusPreInfoGathered

	c := Receiver{
		State:   state,
		PrivKey: privKey,
	}

	return &c, nil
}

func (s *Sender) FundingTxMined(txid string, vout uint32, amount int64, height int) ([]byte, error) {
	s.State.FundingTxID = txid
	s.State.FundingVout = vout
	s.State.FundingAmount = amount
	s.State.BlockHeight = height
	s.State.Status = StatusOpen

	return s.signBalance(0)
}

func (r *Receiver) Open(txid string, vout uint32, amount int64, height int, senderSig []byte) error {
	r.State.FundingTxID = txid
	r.State.FundingVout = vout
	r.State.FundingAmount = amount
	r.State.BlockHeight = height

	if err := r.validateSenderSig(0, senderSig); err != nil {
		return err
	}

	r.SenderSig = senderSig
	r.State.Status = StatusOpen

	return nil
}

func (s *Sender) signBalance(balance int64) ([]byte, error) {
	tx, err := s.State.GetClosureTx(balance)
	if err != nil {
		return nil, err
	}

	script, _, err := s.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	return txscript.RawTxInSignature(
		tx, 0, script, txscript.SigHashAll, s.PrivKey)
}

func (s *Sender) PrepareSend(amount int64) ([]byte, error) {
	newBalance, err := s.State.validateAmount(amount)
	if err != nil {
		return nil, err
	}
	return s.signBalance(newBalance)
}

func (r *Receiver) validateSenderSig(balance int64, senderSig []byte) error {
	rawTx, err := r.State.GetClosureTxSigned(balance, senderSig, r.PrivKey)
	if err != nil {
		return err
	}

	// make sure the sender's sig is valid
	if err := r.State.validateTx(rawTx); err != nil {
		return err
	}

	return nil
}

func (r *Receiver) Send(amount int64, senderSig []byte) error {
	if r.State.Status != StatusOpen {
		return errors.New("channel not open")
	}

	newBalance, err := r.State.validateAmount(amount)
	if err != nil {
		return err
	}

	if err := r.validateSenderSig(newBalance, senderSig); err != nil {
		return err
	}

	// all good, update the state
	// lock
	// if not open, error
	r.State.Count++
	r.State.Balance = newBalance
	r.SenderSig = senderSig
	// unlock

	return nil
}

func (r *Receiver) Close() ([]byte, error) {
	if r.State.Status != StatusOpen && r.State.Status != StatusClosing {
		return nil, errors.New("cannot close channel that isn't open")
	}

	rawTx, err := r.State.GetClosureTxSigned(r.State.Balance, r.SenderSig, r.PrivKey)
	if err != nil {
		return nil, err
	}

	r.State.Status = StatusClosing

	return rawTx, err
}

func (s *Sender) CloseMined() {
	s.State.Status = StatusClosed
}

func (r *Receiver) CloseMined() {
	r.State.Status = StatusClosed
}

func (s *Sender) Refund() ([]byte, error) {
	return s.State.GetRefundTxSigned(s.PrivKey)
}
