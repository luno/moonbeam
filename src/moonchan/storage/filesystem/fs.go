package filesystem

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"moonchan/channels"
	"moonchan/storage"
)

type FilesystemStorage struct {
	dir string
}

func NewFilesystemStorage(dir string) *FilesystemStorage {
	return &FilesystemStorage{
		dir: dir,
	}
}

func (fs *FilesystemStorage) getPath(id string) (string, error) {
	if len(id) == 0 || len(id) > 100 {
		return "", errors.New("invalid id")
	}

	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return "", err
	}

	s := strconv.FormatInt(i, 10)
	if s != id {
		return "", errors.New("invalid id")
	}

	return fs.dir + "/" + id + ".json", nil
}

func (fs *FilesystemStorage) getIdFromPath(path string) (string, error) {
	path = strings.TrimPrefix(path, fs.dir+"/")
	return strings.TrimSuffix(path, ".json"), nil
}

func (fs *FilesystemStorage) Get(id string) (*storage.Record, error) {
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
	paths, err := filepath.Glob(fs.dir + "/*.json")
	if err != nil {
		return nil, err
	}

	var sl []storage.Record
	for _, path := range paths {
		id, err := fs.getIdFromPath(path)
		if err != nil {
			return nil, err
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

func (fs *FilesystemStorage) Create(s channels.SharedState) (string, error) {
	sss, err := s.ToSimple()
	if err != nil {
		return "", err
	}

	n, err := fs.count()
	if err != nil {
		return "", err
	}
	n++
	id := strconv.FormatInt(n, 10)

	path, err := fs.getPath(id)
	if err != nil {
		return "", err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		return "", err
	}

	if err := json.NewEncoder(f).Encode(sss); err != nil {
		f.Close()
		return "", err
	}

	return id, f.Close()
}

func (fs *FilesystemStorage) Update(id string, s channels.SharedState) error {
	sss, err := s.ToSimple()
	if err != nil {
		return err
	}

	path, err := fs.getPath(id)
	if err != nil {
		return err
	}

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

// Make sure FilesystemStorage implements Storage.
var _ storage.Storage = &FilesystemStorage{}
