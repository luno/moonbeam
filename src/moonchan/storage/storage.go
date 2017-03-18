package storage

import (
	"errors"

	"moonchan/channels"
)

var ErrNotFound = errors.New("record not found")

type Record struct {
	ID          string
	SharedState channels.SharedState
}

type Storage interface {
	Get(id string) (*Record, error)
	List() ([]Record, error)
	Create(s channels.SharedState) (string, error)
	Update(id string, s channels.SharedState) error
}
