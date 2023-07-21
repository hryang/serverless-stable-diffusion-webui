package datastore

import "errors"

var ErrNotFound = errors.New("datastore: the key is not found")

type Datastore interface {
	Put(key string, value string) error
	Get(key string) (string, error)
	Delete(key string) error
	Close() error
}
