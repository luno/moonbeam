package models

type CreateRequest struct {
	SenderPubKey []byte
}

type CreateResponse struct {
	ID string

	ReceiverPubKey []byte

	FundingAddress string
}

type OpenRequest struct {
	ID string

	TxID      string
	Vout      uint32
	Amount    int64
	Height    int
	SenderSig []byte
}

type OpenResponse struct {
}

type SendRequest struct {
	ID string

	Amount    int64
	SenderSig []byte
}

type SendResponse struct {
}
