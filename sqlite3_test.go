package sqlite_test

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	_ "go.riyazali.net/sqlite/internal/testing/sqlite"
	"os"
	"testing"
)

// tests' entrypoint that registers the extension
// automatically with all loaded database connections
func TestMain(m *testing.M) { os.Exit(m.Run()) }

// Memory represents a uri to an in-memory database
const Memory = "file:testing.db?mode=memory"

// Connect opens a connection with the sqlite3 database using
// the given data source address and pings it to check liveliness.
func Connect(dataSourceName string) (db *sql.DB, err error) {
	if db, err = sql.Open("sqlite3", dataSourceName); err != nil {
		return nil, err
	} else if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}