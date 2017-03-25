package storage

import (
	"errors"

	"moonchan/channels"
)

var ErrNotFound = errors.New("record not found")

type Record struct {
	ID          int
	SharedState channels.SharedState
}

type Storage interface {
	Get(id int) (*Record, error)
	List() ([]Record, error)
	Create(id int, s channels.SharedState) error
	Update(id int, s channels.SharedState) error

	ReserveKeyPath() (int, error)
}
