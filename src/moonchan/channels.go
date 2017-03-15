package channels

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

type Status int

const (
	StatusNotStarted       = 1
	StatusPreInfoGathered  = 2
	StatusFundingBroadcast = 3
	StatusClosing          = 4
	StatusClosed           = 5
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
}

func genSeed() (*big.Int, error) {
	n := big.NewInt(1)
	n = n.Lsh(n, 256)
	return rand.Int(rand.Reader, n)
}

type Sender struct {
	State SharedState

	RevocationKeys [][]byte

	// Secrets
	PrivKey    *btcec.PrivateKey
	SecretSeed *big.Int
}

func derivePubKey(privKey *btcec.PrivateKey, net *chaincfg.Params) (*btcutil.AddressPubKey, error) {
	pk := (*btcec.PublicKey)(&privKey.PublicKey)
	return btcutil.NewAddressPubKey(pk.SerializeCompressed(), net)
}

func deriveSecret(seed *big.Int, n int64) []byte {
	var r *big.Int
	r.Add(seed, big.NewInt(n))
	buf := []byte(r.String())
	hash := sha256.Sum256(buf)
	return hash[:]
}

func OpenChannel(net *chaincfg.Params, privKey *btcec.PrivateKey) (*Sender, error) {
	pubKey, err := derivePubKey(privKey, net)
	if err != nil {
		return nil, err
	}

	seed, err := genSeed()
	if err != nil {
		return nil, err
	}

	c := Sender{
		State: SharedState{
			Version:      1,
			Net:          net,
			Timeout:      defaultTimeout,
			Fee:          5000,
			Status:       StatusNotStarted,
			SenderPubKey: pubKey,
		},
		PrivKey:    privKey,
		SecretSeed: seed,
	}
	return &c, nil
}

func (s *Sender) ReceivedPubKey(pubKey *btcutil.AddressPubKey) {
	s.State.ReceiverPubKey = pubKey
}

type Receiver struct {
	State SharedState

	RevocationKeys [][]byte

	// Secrets
	PrivKey    *btcec.PrivateKey
	SecretSeed *big.Int
}

func AcceptChannel(state SharedState, privKey *btcec.PrivateKey) (*Receiver, error) {
	pubKey, err := derivePubKey(privKey, state.Net)
	if err != nil {
		return nil, err
	}

	seed, err := genSeed()
	if err != nil {
		return nil, err
	}

	state.ReceiverPubKey = pubKey
	state.Status = StatusPreInfoGathered

	c := Receiver{
		State:      state,
		PrivKey:    privKey,
		SecretSeed: seed,
	}

	return &c, nil
}

func (r *Receiver) FundingTxMined(txid string, vout uint32, amount int64, height int) {
	r.State.FundingTxID = txid
	r.State.FundingVout = vout
	r.State.FundingAmount = amount
	r.State.BlockHeight = height
	r.State.Status = StatusFundingBroadcast
}

func (s *Sender) FundingTxMined(txid string, vout uint32, amount int64, height int) {
	s.State.FundingTxID = txid
	s.State.FundingVout = vout
	s.State.FundingAmount = amount
	s.State.BlockHeight = height
	s.State.Status = StatusFundingBroadcast
}

func (s *Sender) CloseBegin() ([]byte, error) {
	tx, err := s.State.GetClosureTx()
	if err != nil {
		return nil, err
	}

	script, _, err := s.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	sig, err := txscript.RawTxInSignature(
		tx, 0, script, txscript.SigHashAll, s.PrivKey)
	if err != nil {
		return nil, err
	}

	s.State.Status = StatusClosing

	return sig, nil
}

func (r *Receiver) CloseReceived(senderSig []byte) ([]byte, error) {
	tx, err := r.State.GetClosureTx()
	if err != nil {
		return nil, err
	}

	script, _, err := r.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	receiverSig, err := txscript.RawTxInSignature(
		tx, 0, script, txscript.SigHashAll, r.PrivKey)
	if err != nil {
		return nil, err
	}

	rawTx, err := r.State.GetClosureTxSigned(senderSig, receiverSig)

	r.State.Status = StatusClosing

	return rawTx, err
}

func (s *Sender) CloseMined() {
	s.State.Status = StatusClosed
}

func (r *Receiver) CloseMined() {
	r.State.Status = StatusClosed
}

func (s *Sender) Refund() {
}
