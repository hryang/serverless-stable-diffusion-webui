package datastore

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDatastore struct {
	DB *sql.DB
}

func NewSQLiteDatastore(name string) *SQLiteDatastore {
	db, err := sql.Open("sqlite3", name)
	if err != nil {
		panic(fmt.Errorf("failed to open database: %v", err))
	}
	_, err = db.Exec("CREATE TABLE mytable (key text not null primary key, value text);")
	if err != nil {
		panic(fmt.Errorf("failed to create table: %v", err))
	}
	return &SQLiteDatastore{DB: db}
}

func (ds *SQLiteDatastore) Close() error {
	return ds.DB.Close()
}

func (ds *SQLiteDatastore) Get(key string) (string, error) {
	var result string
	err := ds.DB.QueryRow("SELECT value FROM mytable WHERE key = ?", key).Scan(&result)
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrNotFound
		}
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
