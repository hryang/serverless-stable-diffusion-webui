package datastore

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDatastore struct {
	DB *sql.DB
}

func (ds *SQLiteDatastore) Get(key string) (string, error) {
	var result string
	err := ds.DB.QueryRow("SELECT value FROM mytable WHERE key = ?", key).Scan(&result)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (ds *SQLiteDatastore) Put(key string, value string) error {
	_, err := ds.DB.Exec("INSERT OR REPLACE INTO mytable (key, value) VALUES (?, ?)", key, value)
	return err
}

func (ds *SQLiteDatastore) Delete(key string) error {
	_, err := ds.DB.Exec("DELETE FROM mytable WHERE key = ?", key)
	return err
}
