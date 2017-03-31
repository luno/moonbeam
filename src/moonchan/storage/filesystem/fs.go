package filesystem

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"moonchan/channels"
	"moonchan/storage"
)

type data struct {
	KeyPathCounter int
	Channels       map[string]storage.Record
	Payments       []storage.Payment
}

func newData() *data {
	return &data{
		Channels: make(map[string]storage.Record),
		Payments: []storage.Payment{},
	}
}

type FilesystemStorage struct {
	mu   sync.RWMutex
	path string
}

func NewFilesystemStorage(path string) *FilesystemStorage {
	return &FilesystemStorage{
		path: path,
	}
}

func (fs *FilesystemStorage) load() (*data, error) {
	f, err := os.Open(fs.path)
	if os.IsNotExist(err) {
		return newData(), nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	var d data
	if err := json.NewDecoder(f).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (fs *FilesystemStorage) save(d *data) error {
	tmp := fs.path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(d); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, fs.path)
}

func getChannel(d *data, id string) (*storage.Record, error) {
	r, ok := d.Channels[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &r, nil
}

func (fs *FilesystemStorage) Get(id string) (*storage.Record, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	d, err := fs.load()
	if err != nil {
		return nil, err
	}

	return getChannel(d, id)
}

func (fs *FilesystemStorage) List() ([]storage.Record, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	d, err := fs.load()
	if err != nil {
		return nil, err
	}

	var sl []storage.Record
	for id, _ := range d.Channels {
		r, err := getChannel(d, id)
		if err != nil {
			return nil, err
		}
		sl = append(sl, *r)
	}

	return sl, nil
}

func (fs *FilesystemStorage) Create(rec storage.Record) error {
	if rec.ID == "" {
		return errors.New("invalid id")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return err
	}

	if _, ok := d.Channels[rec.ID]; ok {
		return errors.New("record already exists")
	}

	d.Channels[rec.ID] = rec

	return fs.save(d)
}

func checkSame(d *data, id string, prev channels.SimpleSharedState) bool {
	s := d.Channels[id].SharedState
	return s.Status == prev.Status && s.Count == prev.Count
}

func (fs *FilesystemStorage) Update(id string, prev, new channels.SimpleSharedState) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return err
	}

	if _, ok := d.Channels[id]; !ok {
		return storage.ErrNotFound
	}

	if !checkSame(d, id, prev) {
		return storage.ErrConcurrentUpdate
	}

	rec := d.Channels[id]
	rec.SharedState = new
	d.Channels[id] = rec

	return fs.save(d)
}

func (fs *FilesystemStorage) Send(id string, prev, new channels.SimpleSharedState, p storage.Payment) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return err
	}

	if _, ok := d.Channels[id]; !ok {
		return storage.ErrNotFound
	}

	if !checkSame(d, id, prev) {
		return storage.ErrConcurrentUpdate
	}

	rec := d.Channels[id]
	rec.SharedState = new
	d.Channels[id] = rec
	d.Payments = append(d.Payments, p)

	return fs.save(d)
}

func (fs *FilesystemStorage) ReserveKeyPath() (int, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return 0, err
	}

	d.KeyPathCounter++

	return d.KeyPathCounter, fs.save(d)
}

func (fs *FilesystemStorage) ListPayments() ([]storage.Payment, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return nil, err
	}

	return d.Payments, nil
}

// Make sure FilesystemStorage implements Storage.
var _ storage.Storage = &FilesystemStorage{}
