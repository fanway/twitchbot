package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func Connect() *sql.DB {
	db, err := sql.Open("sqlite3", "./data.db?_busy_timeout=5000&cache=shared&mode=rwc")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	return db
}
