package memory

import (
	"strconv"
	"sync"

	"moonchan/channels"
	"moonchan/storage"
)

type MemoryStorage struct {
	mu      sync.Mutex
	records map[string]*storage.Record
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		records: make(map[string]*storage.Record),
	}
}

func (ms *MemoryStorage) Get(id string) (*storage.Record, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	r, ok := ms.records[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return r, nil
}

func (ms *MemoryStorage) List() ([]storage.Record, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	var sl []storage.Record
	for _, r := range ms.records {
		sl = append(sl, *r)
	}

	return sl, nil
}

func (ms *MemoryStorage) Create(s channels.SharedState) (string, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	id := strconv.Itoa(len(ms.records) + 1)
	r := storage.Record{
		ID:          id,
		SharedState: s,
	}
	ms.records[id] = &r

	return id, nil
}

func (ms *MemoryStorage) Update(id string, s channels.SharedState) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	r, ok := ms.records[id]
	if !ok {
		return storage.ErrNotFound
	}

	r.SharedState = s

	return nil
}

// Make sure MemoryStorage implements Storage.
var _ storage.Storage = &MemoryStorage{}
