package sqlite_test

import (
	"database/sql"
	. "go.riyazali.net/sqlite"
	"strings"
	"testing"
)

// caseInsensitive collation sequence matcher
var caseInsensitive = func(a, b string) int {
	if strings.EqualFold(a, b) {
		return 0
	} else {
		return 1
	}
}

func TestCollation(t *testing.T) {
	var err error
	Register(func(api *ExtensionApi) (_ ErrorCode, err error) {
		if err = api.CreateCollation("no_case", caseInsensitive); err != nil {
			return SQLITE_ERROR, err
		}
		var conn = api.Connection()
		if err = conn.Exec("CREATE TABLE x (value TEXT)", nil); err != nil {
			return SQLITE_ERROR, err
		}

		var stmt *Stmt
		if stmt, _, err = conn.Prepare("INSERT INTO x VALUES ($1)"); err != nil {
			return SQLITE_ERROR, err
		}

		for _, v := range []string{"aa", "aA", "Aa", "AA", "bb"} {
			stmt.BindText(1, v)
			if _, err = stmt.Step(); err == nil {
				err = stmt.Reset()
			}
			if err != nil {
				return SQLITE_ERROR, err
			}
		}

		return SQLITE_OK, nil
	})

	var db *sql.DB
	if db, err = Connect(Memory); err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var stmt *sql.Stmt
	if stmt, err = db.Prepare("SELECT * FROM x where value = 'aa' COLLATE no_case;"); err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	var rows *sql.Rows
	if rows, err = stmt.Query(); err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var count = 0
	for rows.Next() {
		count++
	}

	if count != 4 {
		t.Fatalf("invalid count: got %d", count)
	}
}
