package channels

import (
	"bytes"
	"errors"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// Typical close tx size: 369 bytes
// Typical refund tx size: 297 bytes

const dustThreshold = 546

func fundingTxScript(senderPubKey, receiverPubKey *btcutil.AddressPubKey, timeout int64) ([]byte, error) {
	b := txscript.NewScriptBuilder()
	b.AddOp(txscript.OP_IF)
	b.AddInt64(2)
	b.AddData(senderPubKey.ScriptAddress())
	b.AddData(receiverPubKey.ScriptAddress())
	b.AddInt64(2)
	b.AddOp(txscript.OP_CHECKMULTISIG)
	b.AddOp(txscript.OP_ELSE)
	b.AddInt64(timeout)
	b.AddOp(txscript.OP_CHECKSEQUENCEVERIFY)
	b.AddOp(txscript.OP_DROP)
	b.AddOp(txscript.OP_DUP)
	b.AddOp(txscript.OP_HASH160)
	b.AddData(senderPubKey.AddressPubKeyHash().ScriptAddress())
	b.AddOp(txscript.OP_EQUALVERIFY)
	b.AddOp(txscript.OP_CHECKSIG)
	b.AddOp(txscript.OP_ENDIF)
	return b.Script()
}

func (s *SharedState) GetFundingScript() ([]byte, string, error) {
	senderPubKey, err := s.SenderAddressPubKey()
	if err != nil {
		return nil, "", err
	}
	receiverPubKey, err := s.ReceiverAddressPubKey()
	if err != nil {
		return nil, "", err
	}
	net, err := s.GetNet()
	if err != nil {
		return nil, "", err
	}

	script, err := fundingTxScript(senderPubKey, receiverPubKey, s.Timeout)
	if err != nil {
		return nil, "", err
	}

	scriptHash, err := btcutil.NewAddressScriptHash(script, net)
	if err != nil {
		return nil, "", err
	}

	return script, scriptHash.String(), nil
}

func (s *SharedState) spendFundingTx() (*wire.MsgTx, error) {
	txid, err := chainhash.NewHashFromStr(s.FundingTxID)
	if err != nil {
		return nil, err
	}
	txin := wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *txid,
			Index: s.FundingVout,
		},
	}

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&txin)
	return tx, nil
}

func sendToAddress(net *chaincfg.Params, amount int64, addr string) (*wire.TxOut, error) {
	address, err := btcutil.DecodeAddress(addr, net)
	if err != nil {
		return nil, err
	}

	pkscript, err := txscript.PayToAddrScript(address)
	if err != nil {
		return nil, err
	}

	return &wire.TxOut{
		Value:    amount,
		PkScript: pkscript,
	}, nil
}

func (s *SharedState) GetClosureTx(balance int64) (*wire.MsgTx, error) {
	net, err := s.GetNet()
	if err != nil {
		return nil, err
	}

	receiveAmount := balance
	senderAmount := s.Capacity - balance - s.Fee

	tx, err := s.spendFundingTx()
	if err != nil {
		return nil, err
	}

	if receiveAmount >= dustThreshold {
		txout, err := sendToAddress(net, receiveAmount, s.ReceiverOutput)
		if err != nil {
			return nil, err
		}
		tx.AddTxOut(txout)
	}

	if senderAmount >= dustThreshold {
		txout, err := sendToAddress(net, senderAmount, s.SenderOutput)
		if err != nil {
			return nil, err
		}
		tx.AddTxOut(txout)
	}

	return tx, nil
}

func (s *SharedState) GetClosureTxSigned(balance int64, senderSig []byte, privKey *btcec.PrivateKey) ([]byte, error) {
	tx, err := s.GetClosureTx(balance)
	if err != nil {
		return nil, err
	}

	script, _, err := s.GetFundingScript()
	if err != nil {
		return nil, err
	}

	receiverSig, err := txscript.RawTxInSignature(
		tx, 0, script, txscript.SigHashAll, privKey)
	if err != nil {
		return nil, err
	}

	b := txscript.NewScriptBuilder()
	b.AddOp(txscript.OP_FALSE)
	b.AddData(senderSig)
	b.AddData(receiverSig)
	b.AddOp(txscript.OP_TRUE)
	b.AddData(script)
	finalScript, err := b.Script()
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].SignatureScript = finalScript

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SharedState) GetRefundTxSigned(privKey *btcec.PrivateKey) ([]byte, error) {
	net, err := s.GetNet()
	if err != nil {
		return nil, err
	}

	tx, err := s.spendFundingTx()
	if err != nil {
		return nil, err
	}

	amount := s.Capacity - s.Fee
	txout, err := sendToAddress(net, amount, s.SenderOutput)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(txout)

	tx.TxIn[0].Sequence = uint32(s.Timeout)

	script, _, err := s.GetFundingScript()
	if err != nil {
		return nil, err
	}

	sig, err := txscript.RawTxInSignature(
		tx, 0, script, txscript.SigHashAll, privKey)
	if err != nil {
		return nil, err
	}

	senderPubKey, err := s.SenderAddressPubKey()
	if err != nil {
		return nil, err
	}

	b := txscript.NewScriptBuilder()
	b.AddData(sig)
	b.AddData(senderPubKey.ScriptAddress())
	b.AddOp(txscript.OP_FALSE)
	b.AddData(script)
	finalScript, err := b.Script()
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].SignatureScript = finalScript

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SharedState) validateTx(rawTx []byte) error {
	senderPubKey, err := s.SenderAddressPubKey()
	if err != nil {
		return err
	}
	receiverPubKey, err := s.ReceiverAddressPubKey()
	if err != nil {
		return err
	}
	net, err := s.GetNet()
	if err != nil {
		return err
	}

	script, err := fundingTxScript(senderPubKey, receiverPubKey, s.Timeout)
	if err != nil {
		return err
	}
	addr, err := btcutil.NewAddressScriptHash(script, net)
	if err != nil {
		return err
	}
	pkscript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return err
	}

	var tx wire.MsgTx
	if err := tx.BtcDecode(bytes.NewReader(rawTx), 2); err != nil {
		return err
	}

	if len(tx.TxIn) != 1 {
		return errors.New("wrong number of inputs")
	}

	engine, err := txscript.NewEngine(pkscript, &tx, 0, txscript.StandardVerifyFlags, nil)
	if err != nil {
		return err
	}
	if err := engine.Execute(); err != nil {
		return err
	}

	// The transaction must be "standard" otherwise it won't be relayed.
	if len(rawTx) >= mempool.MaxStandardTxSize {
		return errors.New("tx too big")
	}
	for _, txout := range tx.TxOut {
		if txout.Value < dustThreshold {
			return errors.New("dust output")
		}

		sc := txscript.GetScriptClass(txout.PkScript)
		if sc != txscript.PubKeyHashTy && sc != txscript.ScriptHashTy {
			return errors.New("unsupported tx out script class")
		}
	}

	return nil
}
