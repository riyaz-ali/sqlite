package sqlite

// #include <stdlib.h>
// #include <sqlite3ext.h>
// #include "bridge.h"
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
	"reflect"
	"sync"
	"unsafe"
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

	return errorIfNotOk(res)
}

// CreateCollation creates a new collation with the given name using the supplied comparison function.
// The comparison function must obey the rules defined at https://www.sqlite.org/c3ref/create_collation.html
func (ext *ExtensionApi) CreateCollation(name string, cmp func(string, string) int) error {
	var cname = C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var pApp = pointer.Save(cmp)
	var compare = (*[0]byte)(C.collation_function_compare_tramp)
	var destroy = (*[0]byte)(C.function_destroy)

	var res = C._sqlite3_create_collation_v2(ext.db, cname, C.SQLITE_UTF8, pApp, compare, destroy)
	if err := ErrorCode(res); !err.ok() {
		// release pApp as destroy isn't called automatically by sqlite3_create_collation_v2
		pointer.Unref(pApp)
		return err
	}

	return nil
}

func toValues(count C.int, va **C.sqlite3_value) []Value {
	var n = int(count)
	var values []Value
	if n > 0 {
		values = *(*[]Value)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(va)), Len: n, Cap: n}))
		values = values[:n:n]
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
