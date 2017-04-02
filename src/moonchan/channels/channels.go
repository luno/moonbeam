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
	DefaultTimeout = 144
	CloseWindow    = 36

	// MinFundingConf is the minimum number of confirmations required before
	// the funding transaction can be accepted to open a channel.
	MinFundingConf = 1
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

func DefaultState(net *chaincfg.Params) SharedState {
	return SharedState{
		Version: 1,
		Net:     netName(net),
		Timeout: DefaultTimeout,
		Fee:     75000,
		Status:  StatusCreated,
	}
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

func OpenChannel(net *chaincfg.Params, privKey *btcec.PrivateKey, outputAddr string) (*Sender, error) {
	if err := checkSupportedAddress(net, outputAddr); err != nil {
		return nil, err
	}

	pubKey, err := derivePubKey(privKey, net)
	if err != nil {
		return nil, err
	}

	ss := DefaultState(net)
	ss.SenderPubKey = pubKey.PubKey().SerializeCompressed()
	ss.SenderOutput = outputAddr

	c := Sender{
		State:   ss,
		PrivKey: privKey,
	}
	return &c, nil
}

const (
	minTimeout = 100
	maxTimeout = 200
	minFee     = 10000
	maxFee     = 100000
)

func (s *Sender) ReceivedPubKey(receiverPubKey []byte, receiverOutput string, timeout, fee int64) error {
	net, err := s.State.GetNet()
	if err != nil {
		return err
	}
	if err := checkSupportedAddress(net, receiverOutput); err != nil {
		return err
	}

	if timeout < minTimeout {
		return errors.New("timeout is too small")
	}
	if timeout > maxTimeout {
		return errors.New("timeout is too large")
	}
	if fee < minFee {
		return errors.New("fee is too small")
	}
	if fee > maxFee {
		return errors.New("fee is too large")
	}

	if _, err := btcutil.NewAddressPubKey(receiverPubKey, net); err != nil {
		return err
	}

	s.State.Timeout = timeout
	s.State.Fee = fee
	s.State.ReceiverPubKey = receiverPubKey
	s.State.ReceiverOutput = receiverOutput

	return nil
}

type Receiver struct {
	State   SharedState
	PrivKey *btcec.PrivateKey
}

func NewReceiver(state SharedState, privKey *btcec.PrivateKey) (*Receiver, error) {
	return &Receiver{state, privKey}, nil
}

func AcceptChannel(state SharedState, privKey *btcec.PrivateKey) (*Receiver, error) {
	net, err := state.GetNet()
	if err != nil {
		return nil, err
	}
	if err := checkSupportedAddress(net, state.SenderOutput); err != nil {
		return nil, err
	}

	pubKey, err := derivePubKey(privKey, net)
	if err != nil {
		return nil, err
	}

	state.ReceiverPubKey = pubKey.PubKey().SerializeCompressed()
	state.Status = StatusCreated

	c := Receiver{
		State:   state,
		PrivKey: privKey,
	}

	return &c, nil
}

var ErrNotStatusCreated = errors.New("channel is not in state created")
var ErrNotStatusOpen = errors.New("channel is not in state open")
var ErrNotStatusClosing = errors.New("channel is not in state closing")

// The caller must check that the (txid, vout) output is unspent and confirmed.
func (s *Sender) FundingTxMined(txid string, vout uint32, amount int64, height int) ([]byte, error) {
	if s.State.Status != StatusCreated {
		return nil, ErrNotStatusCreated
	}

	s.State.FundingTxID = txid
	s.State.FundingVout = vout
	s.State.Capacity = amount
	s.State.BlockHeight = height
	s.State.Status = StatusOpen

	return s.signBalance(0)
}

func (r *Receiver) Open(txid string, vout uint32, amount int64, height int, senderSig []byte) error {
	if r.State.Status != StatusCreated {
		return ErrNotStatusCreated
	}

	minCapacity := r.State.Fee + dustThreshold
	if amount < minCapacity {
		return errors.New("capacity is too low to open channel")
	}

	r.State.FundingTxID = txid
	r.State.FundingVout = vout
	r.State.Capacity = amount
	r.State.BlockHeight = height

	if err := r.validateSenderSig(0, senderSig); err != nil {
		return err
	}

	r.State.SenderSig = senderSig
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
	if s.State.Status != StatusOpen {
		return nil, ErrNotStatusOpen
	}

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

func (r *Receiver) Validate(amount int64, payment []byte) bool {
	if r.State.Status != StatusOpen {
		return false
	}

	if _, err := r.State.validateAmount(amount); err != nil {
		return false
	}

	if !validatePaymentSize(len(payment)) {
		return false
	}

	return true
}

func (r *Receiver) Send(amount int64, payment, senderSig []byte) error {
	if r.State.Status != StatusOpen {
		return ErrNotStatusOpen
	}

	if !r.Validate(amount, payment) {
		return errors.New("invalid payment")
	}

	newBalance, err := r.State.validateAmount(amount)
	if err != nil {
		return err
	}

	if err := r.validateSenderSig(newBalance, senderSig); err != nil {
		return err
	}

	r.State.Count++
	r.State.Balance = newBalance
	r.State.SenderSig = senderSig
	r.State.Payments = append(r.State.Payments, payment)

	return nil
}

func (s *Sender) SendAccepted(amount int64, payment []byte) error {
	if s.State.Status != StatusOpen {
		return ErrNotStatusOpen
	}

	s.State.Count++
	s.State.Balance += amount
	s.State.Payments = append(s.State.Payments, payment)
	return nil
}

func (s *Sender) Close() error {
	if s.State.Status != StatusOpen && s.State.Status != StatusClosing {
		return ErrNotStatusOpen
	}

	s.State.Status = StatusClosing
	return nil
}

func (r *Receiver) Close() ([]byte, error) {
	if r.State.Status != StatusOpen && r.State.Status != StatusClosing {
		return nil, ErrNotStatusOpen
	}

	rawTx, err := r.State.GetClosureTxSigned(r.State.Balance, r.State.SenderSig, r.PrivKey)
	if err != nil {
		return nil, err
	}

	r.State.Status = StatusClosing

	return rawTx, err
}

func (s *Sender) CloseReceived(rawTx []byte) error {
	if s.State.Status != StatusOpen && s.State.Status != StatusClosing {
		return ErrNotStatusOpen
	}

	if err := s.State.validateTx(rawTx); err != nil {
		return err
	}

	s.State.Status = StatusClosing
	return nil
}

func (s *Sender) CloseMined() error {
	if s.State.Status != StatusClosing {
		return ErrNotStatusClosing
	}
	s.State.Status = StatusClosed
	return nil
}

func (r *Receiver) CloseMined() error {
	if r.State.Status != StatusClosing {
		return ErrNotStatusClosing
	}
	r.State.Status = StatusClosed
	return nil
}

func (s *Sender) Refund() ([]byte, error) {
	return s.State.GetRefundTxSigned(s.PrivKey)
}
