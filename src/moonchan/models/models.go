package models

type CreateRequest struct {
	SenderPubKey []byte
}

type CreateResponse struct {
	ID string

	ReceiverPubKey []byte

	FundingAddress string
}
