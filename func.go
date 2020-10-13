package sqlite

// #include <stdlib.h>
// #include "sqlite3.h"
// #include "bridge/bridge.h"
//
// extern void scalar_function_apply_tramp(sqlite3_context*, int, sqlite3_value**);
// extern void aggregate_function_step_tramp(sqlite3_context*, int, sqlite3_value**);
// extern void aggregate_function_final_tramp(sqlite3_context*);
// extern void window_function_value_tramp(sqlite3_context*);
// extern void window_function_inverse_tramp(sqlite3_context*, int, sqlite3_value**);
// extern int collation_function_compare_tramp(void*, int, char*, int, char*);
// extern void function_destroy(void*);
//
import "C"

import (
	"errors"
	"github.com/mattn/go-pointer"
	"sync"
	"unsafe"
)

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
	C._sqlite3_result_text(ctx.ptr, cv, C.int(len(v)), (*[0]byte)(C.free))
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

var ( // protected store used by aggregate context
	aggregateDataLock  sync.RWMutex
	aggregateDataStore = map[unsafe.Pointer]interface{}{}
)

// AggregateContext is an extension of context that allows us to store custom data related to an execution
type AggregateContext struct {
	*Context
	id unsafe.Pointer // id is an arbitrary pointer that indexes into aggregate data store
}

func (agg *AggregateContext) Data() interface{} {
	aggregateDataLock.RLock()
	defer aggregateDataLock.RUnlock()
	return aggregateDataStore[agg.id]
}

func (agg *AggregateContext) SetData(val interface{}) {
	aggregateDataLock.Lock()
	defer aggregateDataLock.Unlock()
	aggregateDataStore[agg.id] = val
}

// Function represents a base "abstract" sql function.
// Function by itself is not valid. Implementers must pick one of the "sub-types"
// to implement.
type Function interface {
	// Deterministic returns true if the function will always return
	// the same result given the same inputs within a single SQL statement
	Deterministic() bool

	// Args returns the number of arguments that this function accepts
	Args() int
}

// ScalarFunction represents a custom sql scalar function
type ScalarFunction interface {
	Function

	Apply(*Context, ...Value)
}

// AggregateFunction represents a custom sql aggregate function
type AggregateFunction interface {
	Function

	Step(*AggregateContext, ...Value)
	Final(*AggregateContext)
}

// WindowFunction represents a custom sql window function
type WindowFunction interface {
	AggregateFunction

	Value(*AggregateContext)
	Inverse(*AggregateContext, ...Value)
}

// CreateFunction creates a new custom sql function with the given name
func (ext *ExtensionApi) CreateFunction(name string, fn Function) error {
	var cname = C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var eTextRep = C.int(C.SQLITE_UTF8)
	if fn.Deterministic() {
		eTextRep |= C.SQLITE_DETERMINISTIC
	}

	var pApp = pointer.Save(fn)
	var destroy = (*[0]byte)(C.function_destroy)

	var res C.int
	if _, ok := fn.(ScalarFunction); ok {
		var applyTramp = (*[0]byte)(C.scalar_function_apply_tramp)
		res = C._sqlite3_create_function_v2(ext.db, cname, C.int(fn.Args()), eTextRep, pApp, applyTramp, nil, nil, destroy)
	} else if _, ok := fn.(AggregateFunction); ok {
		var stepTramp = (*[0]byte)(C.aggregate_function_step_tramp)
		var finalTramp = (*[0]byte)(C.aggregate_function_final_tramp)

		if _, isWindow := fn.(WindowFunction); !isWindow {
			res = C._sqlite3_create_function_v2(ext.db, cname, C.int(fn.Args()), eTextRep, pApp, nil, stepTramp, finalTramp, destroy)
		} else {
			var valueTramp = (*[0]byte)(C.window_function_value_tramp)
			var inverseTramp = (*[0]byte)(C.window_function_inverse_tramp)
			res = C._sqlite3_create_window_function(ext.db, cname, C.int(fn.Args()), eTextRep, pApp, stepTramp, finalTramp, valueTramp, inverseTramp, destroy)
		}
	} else {
		pointer.Unref(pApp)
		return errors.New("sqlite: unknown function type")
	}

	if ErrorCode(res) == SQLITE_OK {
		return nil
	}
	return ErrorCode(res)
}

// CreateCollation creates a new collation with the given name using the supplied comparison function.
// The comparison function must obey the rules defined at https://www.sqlite.org/c3ref/create_collation.html
func (ext *ExtensionApi) CreateCollation(name string, cmp func(string, string) int) error {
	var cname = C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var pApp = pointer.Save(cmp)
	var compare = (*[0]byte)(C.collation_function_compare_tramp)
	var destroy = (*[0]byte)(C.function_destroy)

	if res := C._sqlite3_create_collation_v2(ext.db, cname, C.SQLITE_UTF8, pApp, compare, destroy); ErrorCode(res) == SQLITE_OK {
		return nil
	} else {
		// release pApp as destroy isn't called automatically by sqlite3_create_collation_v2
		pointer.Unref(pApp)
		return ErrorCode(res)
	}
}

func toValues(count C.int, va **C.sqlite3_value) []Value {
	var n = int(count)
	var values []Value
	if n > 0 {
		values = (*[127]Value)(unsafe.Pointer(va))[:n:n]
	}
	return values
}

func getFunction(ctx *C.sqlite3_context) Function {
	var p = unsafe.Pointer(C._sqlite3_user_data(ctx))
	return pointer.Restore(p).(Function)
}

// C <=> Go trampolines!

//export scalar_function_apply_tramp
func scalar_function_apply_tramp(ctx *C.sqlite3_context, n C.int, v **C.sqlite3_value) {
	getFunction(ctx).(ScalarFunction).Apply(&Context{ptr: ctx}, toValues(n, v)...)
}

//export aggregate_function_step_tramp
func aggregate_function_step_tramp(ctx *C.sqlite3_context, n C.int, v **C.sqlite3_value) {
	var id unsafe.Pointer = C._sqlite3_aggregate_context(ctx, C.int(1))
	var c = &AggregateContext{Context: &Context{ptr: ctx}, id: id}
	getFunction(ctx).(AggregateFunction).Step(c, toValues(n, v)...)
}

//export aggregate_function_final_tramp
func aggregate_function_final_tramp(ctx *C.sqlite3_context) {
	var id unsafe.Pointer = C._sqlite3_aggregate_context(ctx, C.int(0))
	defer func() { aggregateDataLock.Lock(); delete(aggregateDataStore, id); aggregateDataLock.Unlock() }() // release context value

	var c = &AggregateContext{Context: &Context{ptr: ctx}, id: id}
	getFunction(ctx).(AggregateFunction).Final(c)
}

//export window_function_value_tramp
func window_function_value_tramp(ctx *C.sqlite3_context) {
	var id unsafe.Pointer = C._sqlite3_aggregate_context(ctx, C.int(1))
	var c = &AggregateContext{Context: &Context{ptr: ctx}, id: id}
	getFunction(ctx).(WindowFunction).Value(c)
}

//export window_function_inverse_tramp
func window_function_inverse_tramp(ctx *C.sqlite3_context, n C.int, v **C.sqlite3_value) {
	var id unsafe.Pointer = C._sqlite3_aggregate_context(ctx, C.int(1))
	var c = &AggregateContext{Context: &Context{ptr: ctx}, id: id}
	getFunction(ctx).(WindowFunction).Inverse(c, toValues(n, v)...)
}

//export collation_function_compare_tramp
func collation_function_compare_tramp(pApp unsafe.Pointer, aLen C.int, a *C.char, bLen C.int, b *C.char) C.int {
	var fn = pointer.Restore(pApp).(func(string, string) int)
	return C.int(fn(C.GoStringN(a, aLen), C.GoStringN(b, bLen)))
}

//export function_destroy
func function_destroy(ptr unsafe.Pointer) { pointer.Unref(ptr) }
