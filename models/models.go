package models

type CreateRequest struct {
	Version int    `json:"version"`
	Net     string `json:"net"`

	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`
}

type CreateResponse struct {
	Version int    `json:"version"`
	Net     string `json:"net"`
	Timeout int64  `json:"timeout"`
	Fee     int64  `json:"fee"`

	ReceiverPubKey []byte `json:"receiverPubKey"`
	ReceiverOutput string `json:"receiverOutput"`

	FundingAddress string `json:"fundingAddress"`

	ReceiverData []byte `json:"receiverData"`
}

type OpenRequest struct {
	ReceiverData []byte `json:"receiverData"`

	Version int    `json:"version"`
	Net     string `json:"net"`
	Timeout int64  `json:"timeout"`
	Fee     int64  `json:"fee"`

	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`

	ReceiverPubKey []byte `json:"receiverPubKey"`
	ReceiverOutput string `json:"receiverOutput"`

	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	SenderSig []byte `json:"senderSig"`
}

type OpenResponse struct {
	AuthToken string `json:"authToken"`
}

type Payment struct {
	Amount int64  `json:"amount"`
	Target string `json:"target"`
}

type ValidateRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	Payment []byte `json:"payment"`
}

type ValidateResponse struct {
	Valid bool `json:"valid"`
}

type SendRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	Payment []byte `json:"payment"`

	SenderSig []byte `json:"senderSig"`
}

type SendResponse struct {
}

type CloseRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

type CloseResponse struct {
	CloseTx []byte `json:"closeTx"`
}

type StatusRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

type StatusResponse struct {
	Status       int    `json:"status"`
	Balance      int64  `json:"balance"`
	PaymentsHash []byte `json:"paymentsHash"`
}
