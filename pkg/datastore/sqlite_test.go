package datastore

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestSQLiteDatastore(t *testing.T) {
	// Putup
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE mytable (key text not null primary key, value text);")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	datastore := &SQLiteDatastore{DB: db}

	// Test Put and Get
	err = datastore.Put("mykey", "myvalue")
	assert.NoError(t, err)

	value, err := datastore.Get("mykey")
	assert.NoError(t, err)
	assert.Equal(t, "myvalue", value)

	// Test Get with non-existent key
	_, err = datastore.Get("non-existent")
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)

	// Test Put with empty key
	err = datastore.Put("", "myvalue")
	assert.NoError(t, err)

	value, err = datastore.Get("")
	assert.NoError(t, err)
	assert.Equal(t, "myvalue", value)

	// Test Delete
	err = datastore.Delete("mykey")
	assert.NoError(t, err)

	_, err = datastore.Get("mykey")
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
}
