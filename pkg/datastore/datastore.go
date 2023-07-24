package datastore

import "errors"

var ErrNotFound = errors.New("datastore: the key is not found")

type DatastoreType int

const (
	SQLite DatastoreType = iota
	MySQL
	TableStore
)

type Config struct {
	Type                 DatastoreType // the datastore type
	DBName               string        // the database name
	TableName            string
	ColumnConfig         map[string]string // map of column name to column type
	PrimaryKeyColumnName string
}

type Datastore interface {
	// Put inserts or updates the column values in the datastore.
	// It takes a key and a map of column names to values, and returns an error if the operation failed.
	Put(key string, values map[string]interface{}) error

	// Get retrieves the column values from the datastore.
	// It takes a key and a slice of column names, and returns a map of column names to values,
	// along with an error if the operation failed.
	// If the key does not exist, the returned map and error are both nil.
	Get(key string, columns []string) (map[string]interface{}, error)

	//Put(key string, value string) error
	//Get(key string) (string, error)

	// Delete removes a value from the datastore.
	// It takes a key, and returns an error if the operation failed.
	// Note: delete a non-existent key will not return an error.
	Delete(key string) error

	// Close close the datastore.
	Close() error
}
