package filesystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"moonchan/channels"
	"moonchan/storage"
)

type metaInfo struct {
	KeyPathCounter int
}

type FilesystemStorage struct {
	mu  sync.RWMutex
	dir string
}

func NewFilesystemStorage(dir string) *FilesystemStorage {
	return &FilesystemStorage{
		dir: dir,
	}
}

func (fs *FilesystemStorage) getPath(id int) (string, error) {
	s := strconv.Itoa(id)
	return fs.dir + "/" + s + ".json", nil
}

func (fs *FilesystemStorage) getIdFromPath(path string) (int, error) {
	path = strings.TrimPrefix(path, fs.dir+"/")
	path = strings.TrimSuffix(path, ".json")
	return strconv.Atoi(path)
}

func (fs *FilesystemStorage) Get(id int) (*storage.Record, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path, err := fs.getPath(id)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, storage.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	var sss channels.SimpleSharedState
	if err := json.NewDecoder(f).Decode(&sss); err != nil {
		return nil, err
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

func (fs *FilesystemStorage) List() ([]storage.Record, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	paths, err := filepath.Glob(fs.dir + "/*.json")
	if err != nil {
		return nil, err
	}

	var sl []storage.Record
	for _, path := range paths {
		id, err := fs.getIdFromPath(path)
		if err != nil {
			continue
		}

		r, err := fs.Get(id)
		if err != nil {
			return nil, err
		}
		sl = append(sl, *r)
	}

	return sl, nil
}

func (fs *FilesystemStorage) count() (int64, error) {
	paths, err := filepath.Glob(fs.dir + "/*.json")
	if err != nil {
		return 0, err
	}
	return int64(len(paths)), nil
}

func (fs *FilesystemStorage) Create(id int, s channels.SharedState) error {
	sss, err := s.ToSimple()
	if err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	path, err := fs.getPath(id)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(f).Encode(sss); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

func (fs *FilesystemStorage) Update(id int, s channels.SharedState) error {
	sss, err := s.ToSimple()
	if err != nil {
		return err
	}

	path, err := fs.getPath(id)
	if err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(f).Encode(sss); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

func loadOrZero(path string) (*metaInfo, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &metaInfo{}, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	var mi metaInfo
	if err := json.NewDecoder(f).Decode(&mi); err != nil {
		return nil, err
	}

	return &mi, nil
}

func (fs *FilesystemStorage) ReserveKeyPath() (int, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.dir + "/metainfo.json"

	mi, err := loadOrZero(path)
	if err != nil {
		return 0, err
	}

	mi.KeyPathCounter++

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(mi); err != nil {
		return 0, err
	}

	if err := f.Close(); err != nil {
		return 0, err
	}

	return mi.KeyPathCounter, os.Rename(tmp, path)
}

// Make sure FilesystemStorage implements Storage.
var _ storage.Storage = &FilesystemStorage{}
