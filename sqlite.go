package sqlite

// #include <stdlib.h>
// #include "sqlite3.h"
// #include "bridge/bridge.h"
import "C"

import "unsafe"

// Context is an *C.struct_sqlite3_context.
// It is used by custom functions to return result values.
// An SQLite context is in no way related to a Go context.Context.
//
// adapted from https://github.com/crawshaw/sqlite/blob/ae45c9066f6e7b62bb7b491a0c7c9659f866ce7c/func.go
type Context struct{ ptr *C.sqlite3_context }

func (ctx Context) ResultInt(v int)        { C._sqlite3_result_int(ctx.ptr, C.int(v)) }
func (ctx Context) ResultInt64(v int64)    { C._sqlite3_result_int64(ctx.ptr, C.sqlite3_int64(v)) }
func (ctx Context) ResultFloat(v float64)  { C._sqlite3_result_double(ctx.ptr, C.double(v)) }
func (ctx Context) ResultNull()            { C._sqlite3_result_null(ctx.ptr) }
func (ctx Context) ResultValue(v Value)    { C._sqlite3_result_value(ctx.ptr, v.ptr) }
func (ctx Context) ResultZeroBlob(n int64) { C._sqlite3_result_zeroblob64(ctx.ptr, C.sqlite3_uint64(n)) }

func (ctx Context) ResultText(v string) {
	var cv *C.char
	if len(v) != 0 {
		cv = C.CString(v)
	}
	C._sqlite3_result_text0(ctx.ptr, cv, C.int(len(v)), (*[0]byte)(C.free))
}

func (ctx Context) ResultError(err error) {
	if err, ok := err.(ErrorCode); ok {
		C._sqlite3_result_error_code(ctx.ptr, C.int(err))
		return
	}
	var errstr = err.Error()
	var cerrstr = C.CString(errstr)
	defer C.free(unsafe.Pointer(cerrstr))
	C._sqlite3_result_error(ctx.ptr, cerrstr, C.int(len(errstr)))
}

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
