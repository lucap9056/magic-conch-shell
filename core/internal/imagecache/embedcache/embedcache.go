package embedcache

import (
	"fmt"
	"os"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	db         *badger.DB
	expiration time.Duration
}

func NewStore(dir string, expiration time.Duration) (*Store, error) {

	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	return &Store{
		db:         db,
		expiration: expiration,
	}, nil
}

func (s *Store) Get(key string) (string, error) {
	var value string
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = string(val)
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			err = os.ErrNotExist
		}
		return "", err
	}

	return value, nil
}

func (s *Store) Set(key string, value string) error {
	err := s.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), []byte(value)).WithTTL(s.expiration)
		return txn.SetEntry(e)
	})
	if err != nil {
		return fmt.Errorf("failed to save to badger: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
