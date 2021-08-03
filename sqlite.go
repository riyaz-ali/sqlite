// Package sqlite provides a Go wrapper over sqlite3's loadable extension interface.
package sqlite

// #include <stdlib.h>
// #include <string.h>
// #include "sqlite3.h"
// #include "unlock_notify.h"
// #include "bridge/bridge.h"
import "C"

import (
	"fmt"
	"reflect"
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
	unlockNote *C._unlock_note // reference to the unlock_note struct used for unlock notification .. defined in blocking_step.h
}

// wrap wraps the provided handle to sqlite3 database, yielding Conn
func wrap(db *C.sqlite3) *Conn {
	var c = &Conn{db: db, unlockNote: C._unlock_note_alloc()}

	// ensure unlock_note is free'd when connection is no longer in use
	runtime.SetFinalizer(c, func(c *Conn) {
		C._unlock_note_free(c.unlockNote)
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
	if err := ErrorCode(res); !err.ok() {
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

// Exec executes an SQLite query without caching the underlying query.
// It is the spiritual equivalent of sqlite3_exec.
func(conn *Conn) Exec(query string, fn func(stmt *Stmt) error, args ...interface{}) (err error) {
	var stmt *Stmt
	var trailingBytes int
	if stmt, trailingBytes, err = conn.Prepare(query); err != nil {
		return err
	}
	defer func() {
		if ferr := stmt.Finalize(); err == nil {
			err = ferr
		}
	}()

	if trailingBytes != 0 {
		return fmt.Errorf("exec: query %q has trailing bytes", query)
	}

	for i, arg := range args {
		i++ // parameters are 1-indexed
		v := reflect.ValueOf(arg)
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			stmt.BindInt64(i, v.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			stmt.BindInt64(i, int64(v.Uint()))
		case reflect.Float32, reflect.Float64:
			stmt.BindFloat(i, v.Float())
		case reflect.String:
			stmt.BindText(i, v.String())
		case reflect.Bool:
			stmt.BindBool(i, v.Bool())
		case reflect.Invalid:
			stmt.BindNull(i)
		default:
			if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
				stmt.BindBytes(i, v.Bytes())
			} else {
				stmt.BindText(i, fmt.Sprintf("%v", arg))
			}
		}
	}
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return err
		}
		if !hasRow {
			break
		}
		if fn != nil {
			if err := fn(stmt); err != nil {
				return err
			}
		}
	}

	return nil
}