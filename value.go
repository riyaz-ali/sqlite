package sqlite

// #include <stdlib.h>
// #include "sqlite3.h"
// #include "bridge/bridge.h"
import "C"

import "unsafe"

// ColumnType are codes for each of the SQLite fundamental data types:
// https://www.sqlite.org/c3ref/c_blob.html
type ColumnType int

const (
	SQLITE_INTEGER = ColumnType(C.SQLITE_INTEGER)
	SQLITE_FLOAT   = ColumnType(C.SQLITE_FLOAT)
	SQLITE_TEXT    = ColumnType(C.SQLITE3_TEXT)
	SQLITE_BLOB    = ColumnType(C.SQLITE_BLOB)
	SQLITE_NULL    = ColumnType(C.SQLITE_NULL)
)

// Value is an *C.sqlite3_value.
// Value represent all values that can be stored in a database table.
// It is used to extract column values from sql queries.
//
// adapted from https://github.com/crawshaw/sqlite/blob/ae45c9066f6e7b62bb7b491a0c7c9659f866ce7c/func.go
type Value struct{ ptr *C.sqlite3_value }

func (v Value) IsNil() bool      { return v.ptr == nil }
func (v Value) Int() int         { return int(C._sqlite3_value_int(v.ptr)) }
func (v Value) Int64() int64     { return int64(C._sqlite3_value_int64(v.ptr)) }
func (v Value) Float() float64   { return float64(C._sqlite3_value_double(v.ptr)) }
func (v Value) Len() int         { return int(C._sqlite3_value_bytes(v.ptr)) }
func (v Value) Type() ColumnType { return ColumnType(C._sqlite3_value_type(v.ptr)) }
func (v Value) Changed() bool    { return int(C._sqlite3_value_nochange(v.ptr)) != 0 }

func (v Value) Text() string {
	ptr := unsafe.Pointer(C._sqlite3_value_text(v.ptr))
	n := v.Len()
	return C.GoStringN((*C.char)(ptr), C.int(n))
}

func (v Value) Blob() []byte {
	ptr := unsafe.Pointer(C._sqlite3_value_blob(v.ptr))
	n := v.Len()
	return C.GoBytes(ptr, C.int(n))
}
