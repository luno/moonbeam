package receiver

import (
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

	ss := channels.SharedState{
		Version:      1,
		Net:          r.Net,
		Timeout:      144,
		Fee:          1000,
		SenderPubKey: senderPubKey,
	}

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

	resp := models.CreateResponse{
		ID:             id,
		ReceiverPubKey: receiverPubKey,
	}
	return &resp, nil
}
