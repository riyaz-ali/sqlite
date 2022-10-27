package sqlite

// #include <stdlib.h>
// #include <sqlite3ext.h>
// #include "bridge.h"
//
// extern void pointer_destructor_hook_tramp(void*);
import "C"

import (
	"unsafe"

	"github.com/mattn/go-pointer"
)

// see: https://sqlite.org/bindptr.html#pointer_types_are_static_strings
var pointerType = C.CString("golang")

// Context is an *C.struct_sqlite3_context.
// It is used by custom functions to return result values.
// An SQLite context is in no way related to a Go context.Context.
//
// adapted from https://github.com/crawshaw/sqlite/blob/ae45c9066f6e7b62bb7b491a0c7c9659f866ce7c/func.go
type Context struct{ ptr *C.sqlite3_context }

func (ctx Context) ResultInt(v int)       { C._sqlite3_result_int(ctx.ptr, C.int(v)) }
func (ctx Context) ResultInt64(v int64)   { C._sqlite3_result_int64(ctx.ptr, C.sqlite3_int64(v)) }
func (ctx Context) ResultFloat(v float64) { C._sqlite3_result_double(ctx.ptr, C.double(v)) }
func (ctx Context) ResultNull()           { C._sqlite3_result_null(ctx.ptr) }
func (ctx Context) ResultValue(v Value)   { C._sqlite3_result_value(ctx.ptr, v.ptr) }
func (ctx Context) ResultZeroBlob(n int64) {
	C._sqlite3_result_zeroblob64(ctx.ptr, C.sqlite3_uint64(n))
}

func (ctx Context) ResultBlob(v []byte) {
	C._sqlite3_result_blob0(ctx.ptr, C.CBytes(v), C.int(len(v)), (*[0]byte)(C.free))
}

func (ctx Context) ResultText(v string) {
	var cv *C.char
	if len(v) != 0 {
		cv = C.CString(v)
	}
	C._sqlite3_result_text0(ctx.ptr, cv, C.int(len(v)), (*[0]byte)(C.free))
}

func (ctx Context) ResultSubType(v int) {
	C._sqlite3_result_subtype(ctx.ptr, C.uint(v))
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

func (ctx Context) ResultPointer(val interface{}) {
	ptr := pointer.Save(val)
	C._sqlite3_result_pointer(ctx.ptr, ptr, pointerType, (*[0]byte)(C.pointer_destructor_hook_tramp))
}

//export pointer_destructor_hook_tramp
func pointer_destructor_hook_tramp(p unsafe.Pointer) { pointer.Unref(p) }
