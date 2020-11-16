package database

import (
	"database/sql"
	"path/filepath"
	"runtime"

	_ "github.com/mattn/go-sqlite3"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	name       = "/data.db"
	settings   = "?_busy_timeout=5000&cache=shared&mode=rwc"
	params     = basepath + name + settings
)

func Connect() *sql.DB {
	db, err := sql.Open("sqlite3", params)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	return db
}
