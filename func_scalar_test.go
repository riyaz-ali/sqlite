package sqlite_test

import (
	"database/sql"
	"strings"
	"testing"

	. "go.riyazali.net/sqlite"
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

const magic = 0xfe

// X(s) is a custom scalar function that returns the same string s,
// but with an added subtype using ResultSubtype. Used with Is_x(s)
// to test subtypes.
type X struct{}

func (m *X) Args() int           { return 1 }
func (m *X) Deterministic() bool { return true }
func (m *X) Apply(ctx *Context, values ...Value) {
	ctx.ResultText(values[0].Text())
	ctx.ResultSubType(magic)
}

// is_x(s) is a custom scalar function that returns 0 or 1, depending
// if s has the same subtype returned by x(s).
type IsX struct{}

func (m *IsX) Args() int           { return 1 }
func (m *IsX) Deterministic() bool { return true }
func (m *IsX) Apply(ctx *Context, values ...Value) {
	st := values[0].SubType()
	if st == magic { 
		ctx.ResultInt(1)
	}else {
		ctx.ResultInt(0)
	}
}

func TestSubtypeFunctions(t *testing.T) {
	var err error

	Register(func(api *ExtensionApi) (ErrorCode, error) {
		if err := api.CreateFunction("x", &X{}); err != nil {
			return SQLITE_ERROR, err
		}
		if err := api.CreateFunction("is_x", &IsX{}); err != nil {
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
	if stmt, err = db.Prepare("SELECT is_x('f'), is_x(x('t'))"); err != nil {
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

	var shouldFalse int
	var shouldTrue int
	if err = rows.Scan(&shouldFalse, &shouldTrue); err != nil {
		t.Fatal(err)
	}

	if shouldFalse != 0 {
		t.Fatalf("is_x('f') should return false: got %d", shouldFalse)
	}

	if shouldTrue != 1 {
		t.Fatalf("is_x(x('t)) should return true: got %d", shouldTrue)
	}
}