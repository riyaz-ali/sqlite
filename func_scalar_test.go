package sqlite_test

import (
	"database/sql"
	. "go.riyazali.net/sqlite"
	"strings"
	"testing"
)

// Upper implements a UPPER(...) sql scalar function
type Upper struct{}

func (m *Upper) Args() int           { return 1 }
func (m *Upper) Deterministic() bool { return true }
func (m *Upper) Apply(ctx *Context, values ...Value) {
	ctx.ResultText(strings.ToUpper(values[0].Text()))
}

func TestScalarFunction(t *testing.T) {
	var err error

	Register(func(api *ExtensionApi) (ErrorCode, error) {
		if err := api.CreateFunction("upper", &Upper{}); err != nil {
			return SQLITE_ERROR, err
		}
		return SQLITE_OK, nil
	})

	var db *sql.DB
	if db, err = Connect(Memory); err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var stmt *sql.Stmt
	if stmt, err = db.Prepare("SELECT upper('sqlite')"); err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var rows *sql.Rows
	if rows, err = stmt.Query(); err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected query to return single row")
	}

	var result string
	if err = rows.Scan(&result); err != nil {
		t.Fatal(err)
	}

	if result != "SQLITE" {
		t.Fatalf("invalid result: got %q", result)
	}
}
