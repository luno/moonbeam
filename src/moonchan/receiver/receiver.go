package receiver

import (
	"bytes"
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcrpcclient"
	"github.com/btcsuite/btcutil"

	"moonchan/channels"
	"moonchan/models"
	"moonchan/storage"
	"moonchan/storage/memory"
)

type Receiver struct {
	Net *chaincfg.Params

	//mu sync.Mutex

	// hdkeychain

	privKey *btcec.PrivateKey

	bc *btcrpcclient.Client

	//Count int

	//Channels map[string]*channels.Receiver

	db storage.Storage
}

func NewReceiver(net *chaincfg.Params, privKey *btcec.PrivateKey, bc *btcrpcclient.Client) *Receiver {
	ms := memory.NewMemoryStorage()

	return &Receiver{
		Net:     net,
		privKey: privKey,
		bc:      bc,
		db:      ms,
	}
}

func (r *Receiver) Get(id string) *channels.SharedState {
	s, err := r.db.Get(id)
	if err != nil {
		return nil
	}
	if s == nil {
		return nil
	}
	return &s.SharedState
}

func (r *Receiver) List() ([]storage.Record, error) {
	return r.db.List()
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

	id, err := r.db.Create(c.State)
	if err != nil {
		return nil, err
	}

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

func (r *Receiver) get(id string) (*channels.Receiver, error) {
	rec, err := r.db.Get(id)
	if err != nil {
		return nil, err
	}

	c, err := channels.NewReceiver(rec.SharedState, r.privKey)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (r *Receiver) Open(req models.OpenRequest) (*models.OpenResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}

	_, addr, err := c.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	amount, conf, err := getTxOut(r.bc, req.TxID, req.Vout, addr)
	if err != nil {
		return nil, err
	}

	if conf < 1 {
		return nil, errors.New("too few confirmations")
	}
	if conf > int(c.State.Timeout)/2 {
		return nil, errors.New("too many confirmations")
	}

	err = c.Open(req.TxID, req.Vout, amount, 0, req.SenderSig)
	if err != nil {
		return nil, err
	}

	// FIXME: concurrent access
	if err := r.db.Update(req.ID, c.State); err != nil {
		return nil, err
	}

	return &models.OpenResponse{}, nil
}

func (r *Receiver) Send(req models.SendRequest) (*models.SendResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}

	if err := c.Send(req.Amount, req.SenderSig); err != nil {
		return nil, err
	}

	// FIXME: concurrent access
	if err := r.db.Update(req.ID, c.State); err != nil {
		return nil, err
	}

	return &models.SendResponse{}, nil
}

func (r *Receiver) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}

	rawTx, err := c.Close()
	if err != nil {
		return nil, err
	}

	// FIXME: concurrent access
	if err := r.db.Update(req.ID, c.State); err != nil {
		return nil, err
	}

	var tx wire.MsgTx
	err = tx.BtcDecode(bytes.NewReader(rawTx), wire.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	if _, err := r.bc.SendRawTransaction(&tx, false); err != nil {
		return nil, err
	}

	return &models.CloseResponse{rawTx}, nil
}
