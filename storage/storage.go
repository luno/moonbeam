package storage

import (
	"errors"

	"github.com/luno/moonbeam/channels"
)

var ErrNotFound = errors.New("record not found")
var ErrConcurrentUpdate = errors.New("concurrent update")

type Record struct {
	ID          string
	KeyPath     int
	SharedState channels.SharedState
}

type Storage interface {
	Get(id string) (*Record, error)
	List() ([]Record, error)
	Create(rec Record) error
	Update(id string, prev, new channels.SharedState, payment []byte) error
	ReserveKeyPath() (int, error)
	ListPayments(channelID string) ([][]byte, error)
}
