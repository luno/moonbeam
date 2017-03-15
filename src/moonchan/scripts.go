package channels

import (
	"bytes"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const (
	OP_CHECKLOCKTIMEVERIFY = 177
	OP_CHECKSEQUENCEVERIFY = 178
)

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
	b.AddOp(OP_CHECKSEQUENCEVERIFY)
	b.AddOp(txscript.OP_DROP)
	b.AddOp(txscript.OP_DUP)
	b.AddOp(txscript.OP_HASH160)
	b.AddData(senderPubKey.AddressPubKeyHash().ScriptAddress())
	b.AddOp(txscript.OP_EQUALVERIFY)
	b.AddOp(txscript.OP_CHECKSIG)
	b.AddOp(txscript.OP_ENDIF)
	return b.Script()
}

func (c *SharedState) GetFundingScript() ([]byte, string, error) {
	script, err := fundingTxScript(c.SenderPubKey, c.ReceiverPubKey, c.Timeout)
	if err != nil {
		return nil, "", err
	}

	scriptHash, err := btcutil.NewAddressScriptHash(script, c.Net)
	if err != nil {
		return nil, "", err
	}

	return script, scriptHash.String(), nil
}

func (s *SharedState) GetClosureTx() (*wire.MsgTx, error) {
	receiveAmount := s.Balance
	senderAmount := s.FundingAmount - s.Balance - s.Fee

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

	if receiveAmount > 0 {
		pkscript1, err := txscript.PayToAddrScript(s.ReceiverPubKey.AddressPubKeyHash())
		if err != nil {
			return nil, err
		}
		txout1 := wire.TxOut{
			Value:    receiveAmount,
			PkScript: pkscript1,
		}
		tx.AddTxOut(&txout1)
	}

	if senderAmount > 0 {
		pkscript2, err := txscript.PayToAddrScript(s.SenderPubKey.AddressPubKeyHash())
		if err != nil {
			return nil, err
		}
		txout2 := wire.TxOut{
			Value:    senderAmount,
			PkScript: pkscript2,
		}
		tx.AddTxOut(&txout2)
	}

	return tx, nil
}

func (s *SharedState) GetClosureTxSigned(senderSig, receiverSig []byte) ([]byte, error) {
	tx, err := s.GetClosureTx()
	if err != nil {
		return nil, err
	}

	script, _, err := s.GetFundingScript()
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
