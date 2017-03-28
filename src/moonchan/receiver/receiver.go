package receiver

import (
	"bytes"
	"encoding/hex"
	"errors"
	"log"
	"strconv"

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
	"moonchan/storage/filesystem"
)

type Receiver struct {
	Net            *chaincfg.Params
	ek             *hdkeychain.ExtendedKey
	bc             *btcrpcclient.Client
	db             storage.Storage
	receiverOutput string
}

func NewReceiver(net *chaincfg.Params, ek *hdkeychain.ExtendedKey, bc *btcrpcclient.Client, destination string) *Receiver {
	ms := filesystem.NewFilesystemStorage("server-state.json")

	return &Receiver{
		Net:            net,
		ek:             ek,
		bc:             bc,
		db:             ms,
		receiverOutput: destination,
	}
}

func (r *Receiver) Get(id string) *channels.SharedState {
	nid, err := strconv.Atoi(id)
	if err != nil {
		return nil
	}

	s, err := r.db.Get(nid)
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

	if err := r.db.Create(n, c.State); err != nil {
		return nil, err
	}

	receiverPubKey := c.State.ReceiverPubKey.PubKey().SerializeCompressed()

	_, addr, err := c.State.GetFundingScript()
	if err != nil {
		return nil, err
	}

	resp := models.CreateResponse{
		ID:             strconv.Itoa(n),
		Timeout:        c.State.Timeout,
		Fee:            c.State.Fee,
		ReceiverPubKey: receiverPubKey,
		ReceiverOutput: ss.ReceiverOutput,
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

func (r *Receiver) get(id string) (int, *channels.Receiver, error) {
	nid, err := strconv.Atoi(id)
	if err != nil {
		return 0, nil, err
	}

	rec, err := r.db.Get(nid)
	if err != nil {
		return 0, nil, err
	}

	privKey, err := r.getKey(nid)
	if err != nil {
		return 0, nil, err
	}

	c, err := channels.NewReceiver(rec.SharedState, privKey)
	if err != nil {
		return 0, nil, err
	}

	return nid, c, nil
}

func (r *Receiver) Open(req models.OpenRequest) (*models.OpenResponse, error) {
	nid, c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}
	prev := c.State

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

	if err := r.db.Update(nid, prev, c.State); err != nil {
		return nil, err
	}

	return &models.OpenResponse{}, nil
}

func (r *Receiver) Send(req models.SendRequest) (*models.SendResponse, error) {
	nid, c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}
	prev := c.State

	if err := c.Send(req.Amount, req.SenderSig); err != nil {
		return nil, err
	}

	p := storage.Payment{
		Target: req.Target,
		Amount: req.Amount,
	}

	if err := r.db.Send(nid, prev, c.State, p); err != nil {
		return nil, err
	}

	return &models.SendResponse{}, nil
}

func (r *Receiver) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	nid, c, err := r.get(req.ID)
	if err != nil {
		return nil, err
	}
	prev := c.State

	rawTx, err := c.Close()
	if err != nil {
		return nil, err
	}

	log.Printf("closeTx: %s", hex.EncodeToString(rawTx))

	if err := r.db.Update(nid, prev, c.State); err != nil {
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
