package receiver

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcrpcclient"
	"github.com/btcsuite/btcutil/hdkeychain"

	"github.com/luno/moonbeam/channels"
	"github.com/luno/moonbeam/models"
	"github.com/luno/moonbeam/storage"
)

type Receiver struct {
	Net            *chaincfg.Params
	ek             *hdkeychain.ExtendedKey
	bc             *btcrpcclient.Client
	db             storage.Storage
	dir            *Directory
	receiverOutput string
	authKey        []byte
	config         channels.ReceiverConfig
}

func NewReceiver(net *chaincfg.Params,
	ek *hdkeychain.ExtendedKey,
	bc *btcrpcclient.Client,
	db storage.Storage,
	dir *Directory,
	destination string,
	authKey string) *Receiver {

	config := channels.DefaultReceiverConfig
	config.Net = net.Name

	return &Receiver{
		Net:            net,
		ek:             ek,
		bc:             bc,
		db:             db,
		dir:            dir,
		receiverOutput: destination,
		authKey:        []byte(authKey),
		config:         config,
	}
}

func (r *Receiver) Get(txid string, vout uint32) *channels.SharedState {
	id := getChannelID(txid, vout)
	rec, err := r.db.Get(id)
	if err != nil {
		return nil
	}
	if rec == nil {
		return nil
	}
	return &rec.SharedState
}

func (r *Receiver) List() ([]storage.Record, error) {
	return r.db.List()
}

func (r *Receiver) ListPayments(txid string, vout uint32) ([][]byte, error) {
	id := getChannelID(txid, vout)
	return r.db.ListPayments(id)
}

func (r *Receiver) issue(txid string, vout uint32) []byte {
	id := getChannelID(txid, vout)
	mac := hmac.New(sha256.New, r.authKey)
	mac.Write([]byte(id))
	return mac.Sum(nil)
}

func (r *Receiver) issueToken(txid string, vout uint32) string {
	return base64.StdEncoding.EncodeToString(r.issue(txid, vout))
}

func (r *Receiver) ValidateToken(txid string, vout uint32, token string) bool {
	actual, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	expected := r.issue(txid, vout)
	return hmac.Equal(actual, expected)
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

func getChannelID(txid string, vout uint32) string {
	return fmt.Sprintf("%s-%d", strings.ToLower(txid), vout)
}

func (r *Receiver) Create(req models.CreateRequest) (*models.CreateResponse, error) {
	// TODO: Periodically rotate privKey by incrementing the child key
	// counter and return the key index in ReceiverData.
	const keyPath = 0
	privKey, err := r.getKey(keyPath)
	if err != nil {
		return nil, err
	}

	c, err := channels.NewReceiver(r.config, r.receiverOutput, privKey)
	if err != nil {
		return nil, err
	}
	resp, err := c.Create(&req)
	if err != nil {
		return nil, err
	}

	resp.ReceiverData = []byte(strconv.Itoa(keyPath))

	return resp, nil
}

func getTxOut(bc *btcrpcclient.Client, txid string, vout uint32) (*wire.TxOut, int, string, error) {

	txhash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return nil, 0, "", err
	}

	txout, err := bc.GetTxOut(txhash, vout, false)
	if err != nil {
		return nil, 0, "", err
	}
	if txout == nil {
		return nil, 0, "", NewExposableError("confirmed utxo not found")
	}

	if txout.Coinbase {
		return nil, 0, "", NewExposableError("cannot use coinbase utxo")
	}

	pkscript, err := hex.DecodeString(txout.ScriptPubKey.Hex)
	if err != nil {
		return nil, 0, "", err
	}

	// yuck
	value := int64(txout.Value * 1e8)

	wtxout := wire.NewTxOut(value, pkscript)

	return wtxout, int(txout.Confirmations), txout.BestBlock, nil
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

	c, err := channels.LoadReceiver(r.config, rec.SharedState, privKey)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (r *Receiver) getPolicy() policy {
	return getPolicy(r.Net)
}

func (r *Receiver) Open(req models.OpenRequest) (*models.OpenResponse, error) {
	if string(req.ReceiverData) != "0" {
		return nil, errors.New("invalid receiverData")
	}

	txout, conf, blockHash, err := getTxOut(r.bc, req.TxID, req.Vout)
	if err != nil {
		return nil, err
	}

	if conf < r.getPolicy().FundingMinConf {
		return nil, NewExposableError("too few confirmations")
	}

	height, err := getHeight(r.bc, blockHash)
	if err != nil {
		return nil, err
	}

	const keyPath = 0
	privKey, err := r.getKey(keyPath)
	if err != nil {
		return nil, err
	}

	c, err := channels.NewReceiver(r.config, r.receiverOutput, privKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.Open(txout, &req)
	if err != nil {
		return nil, err
	}

	c.State.BlockHeight = int(height)
	if conf > r.getPolicy().SoftTimeout {
		c.State.Status = channels.StatusClosing
	}

	id := getChannelID(req.TxID, req.Vout)

	rec := storage.Record{
		ID:          id,
		KeyPath:     keyPath,
		SharedState: c.State,
	}

	if err := r.db.Create(rec); err != nil {
		return nil, err
	}

	resp.AuthToken = r.issueToken(req.TxID, req.Vout)

	return resp, nil
}

func (r *Receiver) validate(c *channels.Receiver, payment []byte) (bool, *models.Payment, error) {
	var p models.Payment
	if err := json.Unmarshal(payment, &p); err != nil {
		return false, nil, errors.New("invalid payment")
	}

	valid, err := c.Validate(p.Amount, payment)
	if err != nil {
		return false, nil, err
	}
	if !valid {
		return false, nil, nil
	}
	has, err := r.dir.HasTarget(p.Target)
	if err != nil {
		return false, nil, err
	}
	if !has {
		return false, nil, nil
	}

	return true, &p, nil
}

func (r *Receiver) Validate(req models.ValidateRequest) (*models.ValidateResponse, error) {
	id := getChannelID(req.TxID, req.Vout)
	c, err := r.get(id)
	if err != nil {
		return nil, err
	}

	valid, _, err := r.validate(c, req.Payment)
	if err != nil {
		return nil, err
	}

	return &models.ValidateResponse{Valid: valid}, nil
}

func (r *Receiver) Send(req models.SendRequest) (*models.SendResponse, error) {
	id := getChannelID(req.TxID, req.Vout)
	c, err := r.get(id)
	if err != nil {
		return nil, err
	}
	prevState := c.State

	valid, p, err := r.validate(c, req.Payment)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("invalid payment")
	}

	resp, err := c.Send(p.Amount, &req)
	if err != nil {
		return nil, err
	}

	newState := c.State

	if err := r.db.Update(id, prevState, newState, req.Payment); err != nil {
		return nil, err
	}

	return resp, nil
}

func (r *Receiver) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	id := getChannelID(req.TxID, req.Vout)
	c, err := r.get(id)
	if err != nil {
		return nil, err
	}
	prevState := c.State

	resp, err := c.Close(&req)
	if err != nil {
		return nil, err
	}

	log.Printf("closeTx: %s", hex.EncodeToString(resp.CloseTx))

	newState := c.State

	if err := r.db.Update(id, prevState, newState, nil); err != nil {
		return nil, err
	}

	var tx wire.MsgTx
	err = tx.BtcDecode(bytes.NewReader(resp.CloseTx), wire.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	txid, err := r.bc.SendRawTransaction(&tx, false)
	if err != nil {
		return nil, err
	}
	log.Printf("closeTx txid: %s", txid.String())

	return resp, nil
}

func (r *Receiver) Status(req models.StatusRequest) (*models.StatusResponse, error) {
	id := getChannelID(req.TxID, req.Vout)
	c, err := r.get(id)
	if err != nil {
		return nil, err
	}

	return &models.StatusResponse{
		Status:       int(c.State.Status),
		Balance:      c.State.Balance,
		PaymentsHash: c.State.PaymentsHash[:],
	}, nil
}
