package storage

import (
	"errors"

	"moonchan/channels"
)

var ErrNotFound = errors.New("record not found")
var ErrConcurrentUpdate = errors.New("concurrent update")

type Record struct {
	ID          int
	SharedState channels.SharedState
}

type Payment struct {
	Target string
	Amount int64
}

type Storage interface {
	Get(id int) (*Record, error)
	List() ([]Record, error)
	Create(id int, s channels.SharedState) error
	Update(id int, prev, new channels.SharedState) error
	Send(id int, prev, new channels.SharedState, p Payment) error
	ReserveKeyPath() (int, error)
	ListPayments() ([]Payment, error)
}
