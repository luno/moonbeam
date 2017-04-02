package receiver

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcrpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"

	"moonchan/channels"
	"moonchan/models"
	"moonchan/storage"
)

type Receiver struct {
	Net            *chaincfg.Params
	ek             *hdkeychain.ExtendedKey
	bc             *btcrpcclient.Client
	db             storage.Storage
	dir            *Directory
	receiverOutput string
}

func NewReceiver(net *chaincfg.Params,
	ek *hdkeychain.ExtendedKey,
	bc *btcrpcclient.Client,
	db storage.Storage,
	dir *Directory,
	destination string) *Receiver {

	return &Receiver{
		Net:            net,
		ek:             ek,
		bc:             bc,
		db:             db,
		dir:            dir,
		receiverOutput: destination,
	}
}

func (r *Receiver) Get(id string) *channels.SharedState {
	rec, err := r.db.Get(id)
	if err != nil {
		return nil
	}
	if rec == nil {
		return nil
	}

	ss, err := channels.FromSimple(rec.SharedState)
	if err != nil {
		// FIXME: report error
		return nil
	}
	return ss
}

func (r *Receiver) List() ([]storage.Record, error) {
	return r.db.List()
}

func (r *Receiver) ListPayments() ([]storage.Payment, error) {
	return r.db.ListPayments()
}

func (r *Receiver) getKey(n int) (*btcec.PrivateKey, error) {
	ek, err := r.ek.Child(uint32(n))
	if err != nil {
		return nil, err
	}
	return ek.ECPrivKey()
}

func genChannelID() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (r *Receiver) Create(req models.CreateRequest) (*models.CreateResponse, error) {
	senderPubKey, err := btcutil.NewAddressPubKey(req.SenderPubKey, r.Net)
	if err != nil {
		return nil, err
	}

	ss := channels.DefaultState(r.Net)
	ss.SenderPubKey = senderPubKey
	ss.SenderOutput = req.SenderOutput
	ss.ReceiverOutput = r.receiverOutput

	n, err := r.db.ReserveKeyPath()
	if err != nil {
		return nil, err
	}
	privKey, err := r.getKey(n)
	if err != nil {
		return nil, err
	}

	c, err := channels.AcceptChannel(ss, privKey)
	if err != nil {
		return nil, err
	}

	id, err := genChannelID()
	if err != nil {
		return nil, err
	}

	sss, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	rec := storage.Record{
		ID:          id,
		KeyPath:     n,
		SharedState: *sss,
	}

	if err := r.db.Create(rec); err != nil {
		return nil, err
	}

	_, addr, err := c.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	resp := models.CreateResponse{
		ID:             rec.ID,
		Timeout:        c.State.Timeout,
		Fee:            c.State.Fee,
		ReceiverPubKey: sss.ReceiverPubKey,
		ReceiverOutput: sss.ReceiverOutput,
		FundingAddress: addr,
	}
	return &resp, nil
}

func getTxOut(bc *btcrpcclient.Client,
	txid string, vout uint32, addr string) (int64, int, string, error) {

	txhash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return 0, 0, "", err
	}

	txout, err := bc.GetTxOut(txhash, vout, false)
	if err != nil {
		return 0, 0, "", err
	}
	if txout == nil {
		return 0, 0, "", errors.New("cannot find utxo")
	}

	if txout.Coinbase {
		return 0, 0, "", errors.New("cannot use coinbase")
	}

	if len(txout.ScriptPubKey.Addresses) != 1 {
		return 0, 0, "", errors.New("wrong number of addresses")
	}
	if txout.ScriptPubKey.Addresses[0] != addr {
		return 0, 0, "", errors.New("bad address")
	}

	// yuck
	value := int64(txout.Value * 1e8)

	return value, int(txout.Confirmations), txout.BestBlock, nil
}

func getHeight(bc *btcrpcclient.Client, blockhash string) (int64, error) {
	bh, err := chainhash.NewHashFromStr(blockhash)
	if err != nil {
		return 0, err
	}
	header, err := bc.GetBlockHeaderVerbose(bh)
	if err != nil {
		return 0, err
	}
	return int64(header.Height), nil
}

func (r *Receiver) get(id string) (*channels.Receiver, error) {
	rec, err := r.db.Get(id)
	if err != nil {
		return nil, err
	}

	privKey, err := r.getKey(rec.KeyPath)
	if err != nil {
		return nil, err
	}

	ss, err := channels.FromSimple(rec.SharedState)
	if err != nil {
		return nil, err
	}

	c, err := channels.NewReceiver(*ss, privKey)
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
	prev, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	_, addr, err := c.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	amount, conf, blockHash, err := getTxOut(r.bc, req.TxID, req.Vout, addr)
	if err != nil {
		return nil, err
	}

	if conf < channels.MinFundingConf {
		return nil, errors.New("too few confirmations")
	}
	maxConf := c.State.Timeout - channels.CloseWindow
	if conf > int(maxConf) {
		return nil, errors.New("too many confirmations")
	}

	height, err := getHeight(r.bc, blockHash)
	if err != nil {
		return nil, err
	}

	err = c.Open(req.TxID, req.Vout, amount, int(height), req.SenderSig)
	if err != nil {
		return nil, err
	}

	newState, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	if err := r.db.Update(req.ID, *prev, *newState); err != nil {
		return nil, err
	}

	return &models.OpenResponse{}, nil
}

func (r *Receiver) validate(c *channels.Receiver, p models.Payment) (bool, error) {
	if !c.Validate(p.Amount) {
		return false, nil
	}
	return r.dir.HasTarget(p.Target)
}

func (r *Receiver) Validate(req models.ValidateRequest) (*models.ValidateResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}

	valid, err := r.validate(c, req.Payment)
	if err != nil {
		return nil, err
	}

	return &models.ValidateResponse{Valid: valid}, nil
}

func (r *Receiver) Send(req models.SendRequest) (*models.SendResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}
	prev, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	valid, err := r.validate(c, req.Payment)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("invalid payment")
	}

	if err := c.Send(req.Payment.Amount, req.SenderSig); err != nil {
		return nil, err
	}

	p := storage.Payment{
		Target: req.Payment.Target,
		Amount: req.Payment.Amount,
	}

	newState, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	if err := r.db.Send(req.ID, *prev, *newState, p); err != nil {
		return nil, err
	}

	return &models.SendResponse{}, nil
}

func (r *Receiver) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}
	prev, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	rawTx, err := c.Close()
	if err != nil {
		return nil, err
	}

	log.Printf("closeTx: %s", hex.EncodeToString(rawTx))

	newState, err := c.State.ToSimple()
	if err != nil {
		return nil, err
	}

	if err := r.db.Update(req.ID, *prev, *newState); err != nil {
		return nil, err
	}

	var tx wire.MsgTx
	err = tx.BtcDecode(bytes.NewReader(rawTx), wire.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	txid, err := r.bc.SendRawTransaction(&tx, false)
	if err != nil {
		return nil, err
	}
	log.Printf("closeTx txid: %s", txid.String())

	return &models.CloseResponse{rawTx}, nil
}

func (r *Receiver) Status(req models.StatusRequest) (*models.StatusResponse, error) {
	c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}

	return &models.StatusResponse{
		Status:  int(c.State.Status),
		Balance: c.State.Balance,
	}, nil
}
