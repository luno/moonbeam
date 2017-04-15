package models

import (
	"encoding/base64"
)

type CreateRequest struct {
	Version int    `json:"version"`
	Net     string `json:"net"`

	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`
}

type CreateResponse struct {
	ID string `json:"id"`

	Version int    `json:"version"`
	Net     string `json:"net"`
	Timeout int64  `json:"timeout"`
	Fee     int64  `json:"fee"`

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

type Payment struct {
	Amount int64  `json:"amount"`
	Target string `json:"target"`
}

type ValidateRequest struct {
	ID string `json:"id"`

	Payment []byte `json:"payment"`
}

type ValidateResponse struct {
	Valid bool `json:"valid"`
}

type SendRequest struct {
	ID string `json:"id"`

	Payment []byte `json:"payment"`

	SenderSig []byte `json:"senderSig"`
}

type SendResponse struct {
}

type CloseRequest struct {
	ID string `json:"id"`
}

type CloseResponse struct {
	CloseTx []byte `json:"closeTx"`
}

type StatusRequest struct {
	ID string `json:"id"`
}

type StatusResponse struct {
	Status       int    `json:"status"`
	Balance      int64  `json:"balance"`
	PaymentsHash []byte `json:"paymentsHash"`
}

func ValidateChannelID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil
}
