package storage

import (
	"errors"

	"moonchan/channels"
)

var ErrNotFound = errors.New("record not found")
var ErrConcurrentUpdate = errors.New("concurrent update")

type Record struct {
	ID          string
	KeyPath     int
	SharedState channels.SharedState
}

type Payment struct {
	Target string
	Amount int64
}

type Storage interface {
	Get(id string) (*Record, error)
	List() ([]Record, error)
	Create(rec Record) error
	Update(id string, prev, new channels.SharedState) error
	Send(id string, prev, new channels.SharedState, p Payment) error
	ReserveKeyPath() (int, error)
	ListPayments() ([]Payment, error)
}
