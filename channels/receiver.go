package channels

import (
	"bytes"
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"bitbucket.org/bitx/moonchan/models"
)

type ReceiverConfig struct {
	Net     string
	Timeout int64
	FeeRate int64
}

var DefaultReceiverConfig = ReceiverConfig{
	Net:     NetTestnet3,
	Timeout: 1008,
	FeeRate: 300,
}

type Receiver struct {
	config  ReceiverConfig
	privKey *btcec.PrivateKey
	net     *chaincfg.Params
	State   SharedState
}

func NewReceiver(c ReceiverConfig, receiverOutput string, privKey *btcec.PrivateKey) (*Receiver, error) {
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
	state.ReceiverPubKey = pubKey.PubKey().SerializeCompressed()

	if err := checkSupportedAddress(net, receiverOutput); err != nil {
		return nil, errors.New("invalid receiverOutput")
	}
	state.ReceiverOutput = receiverOutput

	return &Receiver{
		config:  c,
		privKey: privKey,
		net:     net,
		State:   state,
	}, nil
}

func LoadReceiver(c ReceiverConfig, state SharedState, privKey *btcec.PrivateKey) (*Receiver, error) {
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
	if !bytes.Equal(pubKeyBytes, state.ReceiverPubKey) {
		return nil, errors.New("state senderPubKey differs from privKey")
	}

	if err := state.sanityCheck(); err != nil {
		return nil, err
	}

	return &Receiver{
		config:  c,
		privKey: privKey,
		net:     net,
		State:   state,
	}, nil
}

func (r *Receiver) Create(req *models.CreateRequest) (*models.CreateResponse, error) {
	if r.State.Status != StatusCreated {
		return nil, ErrNotStatusCreated
	}

	if req.Version != Version {
		return nil, errors.New("unsupported version")
	}
	if req.Net != r.config.Net {
		return nil, errors.New("unsupported net")
	}
	if err := checkSupportedAddress(r.net, req.SenderOutput); err != nil {
		return nil, errors.New("invalid senderOutput")
	}
	if _, err := btcutil.NewAddressPubKey(req.SenderPubKey, r.net); err != nil {
		return nil, errors.New("invalid senderPubKey")
	}

	s := r.State
	s.Version = Version
	s.Timeout = r.config.Timeout
	s.Fee = r.config.FeeRate * typicalCloseTxSize
	s.SenderOutput = req.SenderOutput
	s.SenderPubKey = req.SenderPubKey

	_, fundingAddr, err := s.GetFundingScript()
	if err != nil {
		return nil, err
	}

	//r.State = s

	return &models.CreateResponse{
		Version:        s.Version,
		Net:            s.Net,
		Timeout:        s.Timeout,
		Fee:            s.Fee,
		ReceiverPubKey: s.ReceiverPubKey,
		ReceiverOutput: s.ReceiverOutput,
		FundingAddress: fundingAddr,
	}, nil
}

// TODO: add nconf param and validate according to config
func (r *Receiver) Open(txout *wire.TxOut, req *models.OpenRequest) (*models.OpenResponse, error) {
	if r.State.Status != StatusCreated {
		return nil, ErrNotStatusCreated
	}

	if txout.Value <= 0 {
		return nil, errors.New("invalid amount")
	}
	if _, err := chainhash.NewHashFromStr(req.TxID); err != nil {
		return nil, errors.New("invalid txid")
	}
	if len(req.SenderSig) == 0 {
		return nil, errors.New("missing senderSig")
	}
	if req.Net != r.config.Net {
		return nil, errors.New("wrong net")
	}
	if !bytes.Equal(req.ReceiverPubKey, r.State.ReceiverPubKey) {
		return nil, errors.New("wrong receiverPubKey")
	}
	if req.ReceiverOutput != r.State.ReceiverOutput {
		return nil, errors.New("wrong receiverOutput")
	}

	s := SharedState{
		Version:        req.Version,
		Net:            req.Net,
		Timeout:        req.Timeout,
		Fee:            req.Fee,
		Status:         StatusOpen,
		SenderPubKey:   req.SenderPubKey,
		ReceiverPubKey: req.ReceiverPubKey,
		SenderOutput:   req.SenderOutput,
		ReceiverOutput: req.ReceiverOutput,
		FundingTxID:    req.TxID,
		FundingVout:    req.Vout,
		Capacity:       txout.Value,
		SenderSig:      req.SenderSig,
	}

	// Make sure txout.PkScript matches the funding address.
	script, _, err := s.GetFundingScript()
	if err != nil {
		return nil, err
	}
	expectedAddr, err := btcutil.NewAddressScriptHash(script, r.net)
	if err != nil {
		return nil, err
	}
	expectedPkScript, err := txscript.PayToAddrScript(expectedAddr)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(txout.PkScript, expectedPkScript) {
		return nil, errors.New("mismatched funding address")
	}

	if err := validateSenderSig(s, r.privKey); err != nil {
		return nil, err
	}

	minFee := r.config.FeeRate * typicalCloseTxSize

	acceptable := s.Version == Version &&
		s.Timeout >= r.config.Timeout &&
		s.Fee >= minFee

	if !acceptable {
		r.State.Status = StatusClosing
	}

	r.State = s

	return &models.OpenResponse{}, nil
}

func (r *Receiver) Validate(amount int64, payment []byte) (bool, error) {
	if r.State.Status != StatusOpen {
		return false, ErrNotStatusOpen
	}

	if _, err := r.State.validateAmount(amount); err != nil {
		return false, nil
	}

	if !validatePaymentSize(len(payment)) {
		return false, nil
	}

	return true, nil
}

func (r *Receiver) Send(amount int64, req *models.SendRequest) (*models.SendResponse, error) {
	if r.State.Status != StatusOpen {
		return nil, ErrNotStatusOpen
	}
	valid, err := r.Validate(amount, req.Payment)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("invalid payment")
	}

	newBalance, err := r.State.validateAmount(amount)
	if err != nil {
		return nil, err
	}

	newHash := chainHash(r.State.PaymentsHash, req.Payment)

	if err := r.validateSenderSig(newBalance, newHash, req.SenderSig); err != nil {
		return nil, err
	}

	r.State.Count++
	r.State.Balance = newBalance
	r.State.PaymentsHash = newHash
	r.State.SenderSig = req.SenderSig
	return &models.SendResponse{}, nil
}

func (r *Receiver) Close(req *models.CloseRequest) (*models.CloseResponse, error) {
	if r.State.Status != StatusOpen && r.State.Status != StatusClosing {
		return nil, ErrNotStatusOpen
	}

	rawTx, err := r.State.GetClosureTxSigned(r.State.Balance, r.State.PaymentsHash, r.State.SenderSig, r.privKey)
	if err != nil {
		return nil, err
	}

	r.State.Status = StatusClosing

	return &models.CloseResponse{
		CloseTx: rawTx,
	}, nil
}

func (r *Receiver) Status(req *models.StatusRequest) (*models.StatusResponse, error) {
	return &models.StatusResponse{
		Status:       int(r.State.Status),
		Balance:      r.State.Balance,
		PaymentsHash: r.State.PaymentsHash[:],
	}, nil
}

func (r *Receiver) CloseMined() error {
	if r.State.Status != StatusClosing {
		return ErrNotStatusClosing
	}
	r.State.Status = StatusClosed
	return nil
}

func (r *Receiver) validateSenderSig(balance int64, hash [32]byte, senderSig []byte) error {
	ss := r.State
	ss.Balance = balance
	ss.PaymentsHash = hash
	ss.SenderSig = senderSig
	return validateSenderSig(ss, r.privKey)
}
