package receiver

import (
	"errors"
	"fmt"
	"sync"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"

	"moonchan/channels"
	"moonchan/models"
)

type Receiver struct {
	Net *chaincfg.Params

	mu sync.Mutex

	// hdkeychain

	privKey *btcec.PrivateKey

	Count int

	Channels map[string]*channels.Receiver
}

func NewReceiver(net *chaincfg.Params, privKey *btcec.PrivateKey) *Receiver {
	return &Receiver{
		Net:      net,
		privKey:  privKey,
		Channels: make(map[string]*channels.Receiver),
	}
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

func (r *Receiver) Open(req models.OpenRequest) (*models.OpenResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.Channels[req.ID]
	if !ok {
		return nil, errors.New("unknown channel")
	}

	err := c.Open(req.TxID, req.Vout, req.Amount, req.Height, req.SenderSig)
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
