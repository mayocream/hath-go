package hath

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/syndtr/goleveldb/leveldb"
)

// ErrNotFound ...
var ErrNotFound = leveldb.ErrNotFound

// StorageConf ...
type StorageConf struct {
	DBFile string `mapstructure:"db_file"`
}

// Storage ...
type Storage struct {
	ldb *leveldb.DB
}

// NewStorage ...
func NewStorage(conf StorageConf) (*Storage, error) {
	zap.S().Infof("open leveldb at: %s", conf.DBFile)
	db, err := leveldb.OpenFile(conf.DBFile, nil)
	if err != nil {
		return nil, err
	}
	return &Storage{
		ldb: db,
	}, nil
}

// GetHVFile content of hvfile
func (s *Storage) GetHVFile(hv *HVFile) (*HVFile, error) {
	data, err := s.ldb.Get([]byte(hv.FileID()), nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	newHV := &HVFile{
		Hash: hv.Hash,
		Type: hv.Type,
		Size: hv.Size,
		Xres: hv.Xres,
		Yres: hv.Yres,
		Data: data,
	}
	return newHV, nil
}

// PutHVFile store file
func (s *Storage) PutHVFile(hv *HVFile, data []byte) error {
	return s.ldb.Put([]byte(hv.FileID()), data, nil)
}
