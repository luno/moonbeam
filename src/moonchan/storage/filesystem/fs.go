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
	Channels       map[int]channels.SimpleSharedState
	Payments       []storage.Payment
}

func newData() *data {
	return &data{
		Channels: make(map[int]channels.SimpleSharedState),
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

func getChannel(d *data, id int) (*storage.Record, error) {
	sss, ok := d.Channels[id]
	if !ok {
		return nil, storage.ErrNotFound
	}

	s, err := channels.FromSimple(sss)
	if err != nil {
		return nil, err
	}

	return &storage.Record{
		ID:          id,
		SharedState: *s,
	}, nil
}

func (fs *FilesystemStorage) Get(id int) (*storage.Record, error) {
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

func (fs *FilesystemStorage) Create(id int, s channels.SharedState) error {
	sss, err := s.ToSimple()
	if err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return err
	}

	if _, ok := d.Channels[id]; ok {
		return errors.New("record already exists")
	}

	d.Channels[id] = *sss

	return fs.save(d)
}

func (fs *FilesystemStorage) Update(id int, s channels.SharedState) error {
	sss, err := s.ToSimple()
	if err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return err
	}

	if _, ok := d.Channels[id]; !ok {
		return storage.ErrNotFound
	}

	d.Channels[id] = *sss

	return fs.save(d)
}

func (fs *FilesystemStorage) Send(id int, s channels.SharedState, p storage.Payment) error {
	sss, err := s.ToSimple()
	if err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	d, err := fs.load()
	if err != nil {
		return err
	}

	if _, ok := d.Channels[id]; !ok {
		return storage.ErrNotFound
	}

	d.Channels[id] = *sss
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
