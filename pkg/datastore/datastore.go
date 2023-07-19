package datastore

type Datastore interface {
	Put(key string, value string) error
	Get(key string) (string, error)
	Delete(key string) error
	Close() error
}
