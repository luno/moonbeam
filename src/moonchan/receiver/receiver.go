package receiver

import (
	"errors"
	"fmt"
	"sync"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcrpcclient"
	"github.com/btcsuite/btcutil"

	"moonchan/channels"
	"moonchan/models"
)

type Receiver struct {
	Net *chaincfg.Params

	mu sync.Mutex

	// hdkeychain

	privKey *btcec.PrivateKey

	bc *btcrpcclient.Client

	Count int

	Channels map[string]*channels.Receiver
}

func NewReceiver(net *chaincfg.Params, privKey *btcec.PrivateKey, bc *btcrpcclient.Client) *Receiver {
	return &Receiver{
		Net:      net,
		privKey:  privKey,
		bc:       bc,
		Channels: make(map[string]*channels.Receiver),
	}
}

func (r *Receiver) Get(id string) *channels.SharedState {
	r.mu.Lock()
	s, ok := r.Channels[id]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return &s.State
}

func (r *Receiver) List() map[string]channels.SharedState {
	ssl := make(map[string]channels.SharedState)

	r.mu.Lock()
	for id, rc := range r.Channels {
		ssl[id] = rc.State
	}
	r.mu.Unlock()

	return ssl
}

func (r *Receiver) Create(req models.CreateRequest) (*models.CreateResponse, error) {
	senderPubKey, err := btcutil.NewAddressPubKey(req.SenderPubKey, r.Net)
	if err != nil {
		return nil, err
	}

	ss := channels.DefaultState(r.Net)
	ss.SenderPubKey = senderPubKey

	c, err := channels.AcceptChannel(ss, r.privKey)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.Count++
	id := fmt.Sprintf("mc%d", r.Count)
	r.Channels[id] = c
	r.mu.Unlock()

	receiverPubKey := c.State.ReceiverPubKey.PubKey().SerializeCompressed()

	_, addr, err := c.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	resp := models.CreateResponse{
		ID:             id,
		ReceiverPubKey: receiverPubKey,
		FundingAddress: addr,
	}
	return &resp, nil
}

func getTxOut(bc *btcrpcclient.Client,
	txid string, vout uint32, addr string) (int64, int, error) {

	txhash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return 0, 0, err
	}

	txout, err := bc.GetTxOut(txhash, vout, false)
	if err != nil {
		return 0, 0, err
	}
	if txout == nil {
		return 0, 0, errors.New("cannot find utxo")
	}

	if txout.Coinbase {
		return 0, 0, errors.New("cannot use coinbase")
	}

	if len(txout.ScriptPubKey.Addresses) != 1 {
		return 0, 0, errors.New("wrong number of addresses")
	}
	if txout.ScriptPubKey.Addresses[0] != addr {
		return 0, 0, errors.New("bad address")
	}

	// yuck
	value := int64(txout.Value * 1e8)

	return value, int(txout.Confirmations), nil
}

func (r *Receiver) Open(req models.OpenRequest) (*models.OpenResponse, error) {

	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.Channels[req.ID]
	if !ok {
		return nil, errors.New("unknown channel")
	}

	_, addr, err := c.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	amount, conf, err := getTxOut(r.bc, req.TxID, req.Vout, addr)
	if err != nil {
		return nil, err
	}

	if conf < 3 {
		return nil, errors.New("too few confirmations")
	}
	if conf > int(c.State.Timeout)/2 {
		return nil, errors.New("too many confirmations")
	}

	err = c.Open(req.TxID, req.Vout, amount, 0, req.SenderSig)
	if err != nil {
		return nil, err
	}

	return &models.OpenResponse{}, nil
}

func (r *Receiver) Send(req models.SendRequest) (*models.SendResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.Channels[req.ID]
	if !ok {
		return nil, errors.New("unknown channel")
	}

	err := c.Send(req.Amount, req.SenderSig)
	if err != nil {
		return nil, err
	}

	return &models.SendResponse{}, nil
}

func (r *Receiver) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.Channels[req.ID]
	if !ok {
		return nil, errors.New("unknown channel")
	}

	rawTx, err := c.Close()
	if err != nil {
		return nil, err
	}

	return &models.CloseResponse{rawTx}, nil
}
