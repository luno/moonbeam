package models

import (
	"encoding/base64"
)

type CreateRequest struct {
	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`
}

type CreateResponse struct {
	ID string `json:"id"`

	Timeout int64 `json:"timeout"`
	Fee     int64 `json:"fee"`

	ReceiverPubKey []byte `json:"receiverPubKey"`
	ReceiverOutput string `json:"receiverOutput"`

	FundingAddress string `json:"fundingAddress"`
}

type OpenRequest struct {
	ID string `json:"id"`

	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	SenderSig []byte `json:"senderSig"`
}

type OpenResponse struct {
}

type SendRequest struct {
	ID string `json:"id"`

	Amount    int64  `json:"amount"`
	SenderSig []byte `json:"senderSig"`

	Target string `json:"target"`
}

type SendResponse struct {
}

type CloseRequest struct {
	ID string `json:"id"`
}

type CloseResponse struct {
	CloseTx []byte `json:"closeTx"`
}

type StatusResponse struct {
	Status  int   `json:”status”`
	Balance int64 `json:”balance”`
}

func ValidateChannelID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil
}
