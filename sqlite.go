// Package sqlite provides a Go wrapper over sqlite3's loadable extension interface.
package sqlite

// #include <stdlib.h>
// #include <string.h>
// #include "sqlite3.h"
// #include "unlock_notify.h"
// #include "bridge/bridge.h"
import "C"

import (
	"runtime"
	"unsafe"
)

// Conn is an open connection to an sqlite3 database.
// Currently, it only provides a subset of methods available
// on an sqlite3 database handle. Namely, it only supports
// building a prepared statement (and operations on a prepared statement)
//
// A Conn can only be used by goroutine at a time.
type Conn struct {
	db         *C.sqlite3     // reference to the underlying sqlite3 database handle
	unlockNote *C.unlock_note // reference to the unlock_note struct used for unlock notification .. defined in blocking_step.h
}

// wrap wraps the provided handle to sqlite3 database, yielding Conn
func wrap(db *C.sqlite3) *Conn {
	var c = &Conn{db: db, unlockNote: C.unlock_note_alloc()}

	// ensure unlock_note is free'd when connection is no longer in use
	runtime.SetFinalizer(c, func(c *Conn) {
		C.free(unsafe.Pointer(c.unlockNote))
	})

	return c
}

// LastInsertRowID reports the rowid of the most recently successful INSERT.
// see: https://www.sqlite.org/c3ref/last_insert_rowid.html
func (conn *Conn) LastInsertRowID() int64 {
	return int64(C._sqlite3_last_insert_rowid(conn.db))
}

// Prepare prepares a query and returns an Stmt.
//
// If the query has any unprocessed trailing bytes, its count is returned.
// see: https://www.sqlite.org/c3ref/prepare.html
func (conn *Conn) Prepare(query string) (*Stmt, int, error) {
	var stmt = &Stmt{
		conn:      conn,
		query:     query,
		bindNames: make(map[string]int),
		colNames:  make(map[string]int),
	}

	var sql = C.CString(query)
	defer C.free(unsafe.Pointer(sql))
	var trailing *C.char

	var res = C._sqlite3_prepare_v2(conn.db, sql, -1, &stmt.stmt, &trailing)
	if err := ErrorCode(res); err != SQLITE_OK {
		return nil, 0, err
	}

	for i, count := 1, stmt.BindParamCount(); i <= count; i++ {
		cname := C._sqlite3_bind_parameter_name(stmt.stmt, C.int(i))
		if cname != nil {
			stmt.bindNames[C.GoString(cname)] = i
		}
	}

	for i, count := 0, stmt.ColumnCount(); i < count; i++ {
		cname := C._sqlite3_column_name(stmt.stmt, C.int(i))
		if cname != nil {
			stmt.colNames[C.GoString(cname)] = i
		}
	}

	return stmt, int(C.strlen(trailing)), nil
}
