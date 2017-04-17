package channels

import (
	"bytes"
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"

	"github.com/luno/moonbeam/models"
)

type SenderConfig struct {
	Net string

	MinTimeout int64
	MaxTimeout int64

	MinFeeRate int64
	MaxFeeRate int64
}

var DefaultSenderConfig = SenderConfig{
	Net:        NetTestnet3,
	MinTimeout: 144,
	MaxTimeout: 1008,
	MinFeeRate: 10,
	MaxFeeRate: 300,
}

type Sender struct {
	config  SenderConfig
	privKey *btcec.PrivateKey
	net     *chaincfg.Params
	State   SharedState
}

func NewSender(c SenderConfig, privKey *btcec.PrivateKey) (*Sender, error) {
	state := SharedState{
		Net:    c.Net,
		Status: StatusCreated,
	}
	net, err := state.GetNet()
	if err != nil {
		return nil, err
	}

	if privKey == nil {
		return nil, errors.New("invalid privKey")
	}
	pubKey, err := derivePubKey(privKey, net)
	if err != nil {
		return nil, err
	}
	state.SenderPubKey = pubKey.PubKey().SerializeCompressed()

	s := Sender{
		config:  c,
		privKey: privKey,
		net:     net,
		State:   state,
	}

	return &s, nil
}

func LoadSender(c SenderConfig, state SharedState, privKey *btcec.PrivateKey) (*Sender, error) {
	if c.Net != state.Net {
		return nil, errors.New("state net differs from config net")
	}

	net, err := state.GetNet()
	if err != nil {
		return nil, err
	}

	if privKey == nil {
		return nil, errors.New("invalid privKey")
	}
	pubKey, err := derivePubKey(privKey, net)
	if err != nil {
		return nil, err
	}
	pubKeyBytes := pubKey.PubKey().SerializeCompressed()
	if !bytes.Equal(pubKeyBytes, state.SenderPubKey) {
		return nil, errors.New("state senderPubKey differs from privKey")
	}

	if err := state.sanityCheck(); err != nil {
		return nil, err
	}

	return &Sender{
		config:  c,
		privKey: privKey,
		net:     net,
		State:   state,
	}, nil
}

func (s *Sender) GetCreateRequest(outputAddr string) (*models.CreateRequest, error) {
	if s.State.Status != StatusCreated {
		return nil, ErrNotStatusCreated
	}

	if err := checkSupportedAddress(s.net, outputAddr); err != nil {
		return nil, err
	}

	s.State.SenderOutput = outputAddr

	return &models.CreateRequest{
		Version:      Version,
		Net:          s.State.Net,
		SenderPubKey: s.State.SenderPubKey,
		SenderOutput: s.State.SenderOutput,
	}, nil
}

func (s *Sender) GotCreateResponse(resp *models.CreateResponse) error {
	if s.State.Status != StatusCreated {
		return ErrNotStatusCreated
	}
	if s.State.SenderOutput == "" {
		return errors.New("senderOutput is missing")
	}

	if resp.Version != Version {
		return errors.New("unsupported version")
	}
	if resp.Net != s.config.Net {
		return errors.New("unsupported net")
	}
	if resp.Timeout < s.config.MinTimeout {
		return errors.New("timeout is too small")
	}
	if resp.Timeout > s.config.MaxTimeout {
		return errors.New("timeout is too large")
	}
	if resp.Fee < typicalCloseTxSize*s.config.MinFeeRate {
		return errors.New("fee is too small")
	}
	if resp.Fee > typicalCloseTxSize*s.config.MaxFeeRate {
		return errors.New("fee is too large")
	}
	if err := checkSupportedAddress(s.net, resp.ReceiverOutput); err != nil {
		return errors.New("invalid receiverOutput")
	}
	if _, err := btcutil.NewAddressPubKey(resp.ReceiverPubKey, s.net); err != nil {
		return errors.New("invalid receiverPubKey")
	}

	newState := s.State
	newState.Version = resp.Version
	newState.Timeout = resp.Timeout
	newState.Fee = resp.Fee
	newState.ReceiverPubKey = resp.ReceiverPubKey
	newState.ReceiverOutput = resp.ReceiverOutput

	_, addr, err := newState.GetFundingScript()
	if err != nil {
		return err
	}
	if addr != resp.FundingAddress {
		return errors.New("funding address mismatch")
	}

	s.State = newState
	return nil
}

func (s *Sender) signBalance(balance int64, hash [32]byte) ([]byte, error) {
	tx, err := s.State.GetClosureTx(balance, hash)
	if err != nil {
		return nil, err
	}

	script, _, err := s.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	return txscript.RawTxInSignature(
		tx, 0, script, txscript.SigHashAll, s.privKey)
}

func (s *Sender) GetOpenRequest(txid string, vout uint32, amount int64) (*models.OpenRequest, error) {
	if s.State.Status != StatusCreated {
		return nil, ErrNotStatusCreated
	}

	if _, err := chainhash.NewHashFromStr(txid); err != nil {
		return nil, errors.New("invalid txid")
	}
	if amount <= 0 {
		return nil, errors.New("invalid amount")
	}

	s.State.FundingTxID = txid
	s.State.FundingVout = vout
	s.State.Capacity = amount

	sig, err := s.signBalance(0, s.State.PaymentsHash)
	if err != nil {
		return nil, err
	}

	return &models.OpenRequest{
		Version: s.State.Version,
		Net:     s.State.Net,
		Timeout: s.State.Timeout,
		Fee:     s.State.Fee,

		SenderPubKey: s.State.SenderPubKey,
		SenderOutput: s.State.SenderOutput,

		ReceiverPubKey: s.State.ReceiverPubKey,
		ReceiverOutput: s.State.ReceiverOutput,

		TxID:      txid,
		Vout:      vout,
		SenderSig: sig,
	}, nil
}

func (s *Sender) GotOpenResponse(resp *models.OpenResponse) error {
	if s.State.Status != StatusCreated {
		return ErrNotStatusCreated
	}
	if s.State.FundingTxID == "" {
		return errors.New("fundingTxID is missing")
	}
	s.State.Status = StatusOpen
	return nil
}

func (s *Sender) GetSendRequest(amount int64, payment []byte) (*models.SendRequest, error) {
	if s.State.Status != StatusOpen {
		return nil, ErrNotStatusOpen
	}

	newBalance, err := s.State.validateAmount(amount)
	if err != nil {
		return nil, err
	}

	if !validatePaymentSize(len(payment)) {
		return nil, errors.New("invalid payment")
	}

	newHash := chainHash(s.State.PaymentsHash, payment)

	sig, err := s.signBalance(newBalance, newHash)
	if err != nil {
		return nil, err
	}

	return &models.SendRequest{
		TxID:      s.State.FundingTxID,
		Vout:      s.State.FundingVout,
		Payment:   payment,
		SenderSig: sig,
	}, nil
}

func (s *Sender) GotSendResponse(amount int64, payment []byte, resp *models.SendResponse) error {
	if s.State.Status != StatusOpen {
		return ErrNotStatusOpen
	}

	newHash := chainHash(s.State.PaymentsHash, payment)

	s.State.Count++
	s.State.Balance += amount
	s.State.PaymentsHash = newHash

	return nil
}

func (s *Sender) GetCloseRequest() (*models.CloseRequest, error) {
	if s.State.Status != StatusOpen && s.State.Status != StatusClosing {
		return nil, ErrNotStatusOpen
	}
	s.State.Status = StatusClosing
	return &models.CloseRequest{
		TxID: s.State.FundingTxID,
		Vout: s.State.FundingVout,
	}, nil
}

func (s *Sender) GotCloseResponse(resp *models.CloseResponse) error {
	if s.State.Status != StatusOpen && s.State.Status != StatusClosing {
		return ErrNotStatusOpen
	}

	if err := s.State.validateTx(resp.CloseTx); err != nil {
		return err
	}

	if s.State.Status == StatusOpen {
		s.State.Status = StatusClosing
	}

	return nil
}

func (s *Sender) Refund() ([]byte, error) {
	return s.State.GetRefundTxSigned(s.privKey)
}

func (s *Sender) CloseMined() error {
	if s.State.Status != StatusClosing {
		return ErrNotStatusClosing
	}
	s.State.Status = StatusClosed
	return nil
}
