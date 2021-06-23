package sqlite

// #include <stdlib.h>
// #include <string.h>
// #include "sqlite3.h"
// #include "unlock_notify.h"
// #include "bridge/bridge.h"
//
// // Use a helper function here to avoid the cgo pointer detection
// // logic treating SQLITE_TRANSIENT as a Go pointer.
// static int transient_bind_blob(sqlite3_stmt* stmt, int col, unsigned char* p, int n) {
//	return _sqlite3_bind_blob(stmt, col, p, n, SQLITE_TRANSIENT);
// }
import "C"

import (
	"bytes"
	"github.com/mattn/go-pointer"
	"reflect"
	"runtime"
	"unsafe"
)

// Stmt is an SQLite3 prepared statement.
//
// A Stmt is attached to a particular Conn
// (and that Conn can only be used by a single goroutine).
//
// When a Stmt is no longer needed it should be cleaned up
// by calling the Finalize method.
type Stmt struct {
	conn       *Conn
	stmt       *C.sqlite3_stmt
	query      string
	bindNames  map[string]int
	colNames   map[string]int
	bindErr    error
	lastHasRow bool // last bool returned by Step
}

// Finalize deletes a prepared statement.
//
// Be sure to always call Finalize when done with
// a statement created using Prepare.
//
// Do not call Finalize on a prepared statement that
// you intend to prepare again in the future.
//
// see: https://www.sqlite.org/c3ref/finalize.html
func (stmt *Stmt) Finalize() error {
	var res = C._sqlite3_finalize(stmt.stmt)
	stmt.conn = nil
	return errorIfNotOk(res)
}

// Reset resets a prepared statement so it can be executed again.
//
// Note that any parameter values bound to the statement are retained.
// To clear bound values, call ClearBindings.
//
// see: https://www.sqlite.org/c3ref/reset.html
func (stmt *Stmt) Reset() error {
	stmt.lastHasRow = false
	var res C.int
	for {
		res = C._sqlite3_reset(stmt.stmt)
		if res != C.SQLITE_LOCKED_SHAREDCACHE {
			break
		}
		// An SQLITE_LOCKED_SHAREDCACHE error has been seen from sqlite3_reset
		// in the wild, but so far has eluded exact test case replication.
		var err = ErrorCode(C.wait_for_unlock_notify(stmt.conn.db, stmt.conn.unlockNote))
		if !err.ok() {
			return err
		}
	}

	return errorIfNotOk(res)
}

// ClearBindings clears all bound parameter values on a statement.
//
// see: https://www.sqlite.org/c3ref/clear_bindings.html
func (stmt *Stmt) ClearBindings() error {
	return errorIfNotOk(C._sqlite3_clear_bindings(stmt.stmt))
}

// Step moves through the statement cursor using sqlite3_step.
//
// If a row of data is available, rowReturned is reported as true.
// If the statement has reached the end of the available data then
// rowReturned is false. Thus the status codes SQLITE_ROW and
// SQLITE_DONE are reported by the rowReturned bool, and all other
// non-OK status codes are reported as an error.
//
// If an error value is returned, then the statement has been reset.
//
// https://www.sqlite.org/c3ref/step.html
//
// Shared cache
//
// If Shared Cache mode is enabled, this Step method uses sqlite3_unlock_notify
// to handle any SQLITE_LOCKED errors.
//
// Without the shared cache, SQLite will block for
// several seconds while trying to acquire the write lock.
// With the shared cache, it returns SQLITE_LOCKED immediately
// if the write lock is held by another connection in this process.
// Dealing with this correctly makes for an unpleasant programming
// experience, so this package does it automatically by blocking
// Step until the write lock is relinquished.
//
// This means Step can block for a very long time.
//
// For far more details, see: http://www.sqlite.org/unlock_notify.html
func (stmt *Stmt) Step() (rowReturned bool, err error) {
	if err = stmt.bindErr; err != nil {
		stmt.bindErr = nil
		_ = stmt.Reset()
		return false, err
	}

	if rowReturned, err = stmt.step(); err != nil {
		C._sqlite3_reset(stmt.stmt)
	}

	stmt.lastHasRow = rowReturned
	return rowReturned, err
}

func (stmt *Stmt) step() (bool, error) {
	for {
		switch res := C._sqlite3_step(stmt.stmt); uint8(res) { // reduce to non-extended error code
		case C.SQLITE_LOCKED:
			if res != C.SQLITE_LOCKED_SHAREDCACHE {
				// don't call wait_for_unlock_notify as it might deadlock, see:
				// see: https://github.com/crawshaw/sqlite/issues/6
				return false, ErrorCode(res)
			}

			if res = C.wait_for_unlock_notify(stmt.conn.db, stmt.conn.unlockNote); res != C.SQLITE_OK {
				return false, ErrorCode(res)
			}
			C._sqlite3_reset(stmt.stmt)
			// loop
		case C.SQLITE_ROW:
			return true, nil
		case C.SQLITE_DONE:
			return false, nil
		default:
			return false, ErrorCode(res)
		}
	}
}

func (stmt *Stmt) handleBindErr(res C.int) {
	if err := ErrorCode(res); !err.ok() && stmt.bindErr == nil {
		stmt.bindErr = err
	}
}

func (stmt *Stmt) findBindName(param string) int {
	pos := stmt.bindNames[param]
	if pos == 0 && stmt.bindErr == nil {
		stmt.bindErr = SQLITE_ERROR
	}
	return pos
}

// DataCount returns the number of columns in the current row of the result
// set of prepared statement.
//
// see: https://sqlite.org/c3ref/data_count.html
func (stmt *Stmt) DataCount() int {
	return int(C._sqlite3_data_count(stmt.stmt))
}

// ColumnCount returns the number of columns in the result set returned by the
// prepared statement.
//
// see: https://sqlite.org/c3ref/column_count.html
func (stmt *Stmt) ColumnCount() int {
	return int(C._sqlite3_column_count(stmt.stmt))
}

// ColumnName returns the name assigned to a particular column in the result
// set of a SELECT statement.
//
// see: https://sqlite.org/c3ref/column_name.html
func (stmt *Stmt) ColumnName(col int) string {
	return C.GoString((*C.char)(unsafe.Pointer(C._sqlite3_column_name(stmt.stmt, C.int(col)))))
}

// BindName returns the name assigned to a particular parameter in the query.
//
// see: https://www.sqlite.org/c3ref/bind_parameter_name.html
func (stmt *Stmt) BindName(param int) string {
	return C.GoString((*C.char)(unsafe.Pointer(C._sqlite3_bind_parameter_name(stmt.stmt, C.int(param)))))
}

// BindParamCount reports the number of parameters in stmt.
//
// see: https://www.sqlite.org/c3ref/bind_parameter_count.html
func (stmt *Stmt) BindParamCount() int {
	if stmt.stmt == nil {
		return 0
	}
	return int(C._sqlite3_bind_parameter_count(stmt.stmt))
}

// BindInt64 binds value to a numbered stmt parameter.
func (stmt *Stmt) BindInt64(param int, value int64) {
	if stmt.stmt == nil {
		return
	}
	res := C._sqlite3_bind_int64(stmt.stmt, C.int(param), C.sqlite3_int64(value))
	stmt.handleBindErr(res)
}

// BindBool binds value (as an integer 0 or 1) to a numbered stmt parameter.
func (stmt *Stmt) BindBool(param int, value bool) {
	if stmt.stmt == nil {
		return
	}
	v := 0
	if value {
		v = 1
	}
	res := C._sqlite3_bind_int64(stmt.stmt, C.int(param), C.sqlite3_int64(v))
	stmt.handleBindErr(res)
}

// BindBytes binds value to a numbered stmt parameter.
// In-memory copies of value are made using this interface.
func (stmt *Stmt) BindBytes(param int, value []byte) {
	if stmt.stmt == nil {
		return
	}
	var v *C.uchar
	if len(value) != 0 {
		v = (*C.uchar)(unsafe.Pointer(&value[0]))
	}
	res := C.transient_bind_blob(stmt.stmt, C.int(param), v, C.int(len(value)))
	runtime.KeepAlive(value)
	stmt.handleBindErr(res)
}

var emptyCstr = C.CString("")

// BindText binds value to a numbered stmt parameter.
func (stmt *Stmt) BindText(param int, value string) {
	if stmt.stmt == nil {
		return
	}
	var v *C.char
	var free *[0]byte
	if len(value) == 0 {
		v = emptyCstr
	} else {
		v = C.CString(value)
		free = (*[0]byte)(C.free)
	}
	res := C._sqlite3_bind_text(stmt.stmt, C.int(param), v, C.int(len(value)), free)
	stmt.handleBindErr(res)
}

// BindFloat binds value to a numbered stmt parameter.
func (stmt *Stmt) BindFloat(param int, value float64) {
	if stmt.stmt == nil {
		return
	}
	res := C._sqlite3_bind_double(stmt.stmt, C.int(param), C.double(value))
	stmt.handleBindErr(res)
}

// BindNull binds an SQL NULL value to a numbered stmt parameter.
func (stmt *Stmt) BindNull(param int) {
	if stmt.stmt == nil {
		return
	}
	res := C._sqlite3_bind_null(stmt.stmt, C.int(param))
	stmt.handleBindErr(res)
}

// BindNull binds a blob of zeros of length len to a numbered stmt parameter.
func (stmt *Stmt) BindZeroBlob(param int, len int64) {
	if stmt.stmt == nil {
		return
	}
	res := C._sqlite3_bind_zeroblob64(stmt.stmt, C.int(param), C.sqlite3_uint64(len))
	stmt.handleBindErr(res)
}

// BindValue binds an sqlite_value object at given index
func (stmt *Stmt) BindValue(param int, value Value) {
	if stmt.stmt == nil {
		return
	}
	res := C._sqlite3_bind_value(stmt.stmt, C.int(param), value.ptr)
	stmt.handleBindErr(res)
}

// BindPointer binds any arbitrary Go value with the parameter.
// The value can later be retrieved by custom functions or callbacks, casted back into a Go type,
// and used in golang's environment.
func (stmt *Stmt) BindPointer(param int, arg interface{}) {
	if stmt.stmt == nil {
		return
	}
	ptr := pointer.Save(arg)
	res := C._sqlite3_bind_pointer(stmt.stmt, C.int(param), ptr, pointerType, (*[0]byte)(C.pointer_destructor_hook_tramp))
	stmt.handleBindErr(res)
}

// SetInt64 binds an int64 to a parameter using a column name.
func (stmt *Stmt) SetInt64(param string, value int64) {
	stmt.BindInt64(stmt.findBindName(param), value)
}

// SetBool binds a value (as a 0 or 1) to a parameter using a column name.
func (stmt *Stmt) SetBool(param string, value bool) {
	stmt.BindBool(stmt.findBindName(param), value)
}

// SetBytes binds bytes to a parameter using a column name.
// An invalid parameter name will cause the call to Step to return an error.
func (stmt *Stmt) SetBytes(param string, value []byte) {
	stmt.BindBytes(stmt.findBindName(param), value)
}

// SetText binds text to a parameter using a column name.
// An invalid parameter name will cause the call to Step to return an error.
func (stmt *Stmt) SetText(param string, value string) {
	stmt.BindText(stmt.findBindName(param), value)
}

// SetFloat binds a float64 to a parameter using a column name.
// An invalid parameter name will cause the call to Step to return an error.
func (stmt *Stmt) SetFloat(param string, value float64) {
	stmt.BindFloat(stmt.findBindName(param), value)
}

// SetNull binds a null to a parameter using a column name.
// An invalid parameter name will cause the call to Step to return an error.
func (stmt *Stmt) SetNull(param string) {
	stmt.BindNull(stmt.findBindName(param))
}

// SetZeroBlob binds a zero blob of length len to a parameter using a column name.
// An invalid parameter name will cause the call to Step to return an error.
func (stmt *Stmt) SetZeroBlob(param string, len int64) {
	stmt.BindZeroBlob(stmt.findBindName(param), len)
}

// SetValue binds an sqlite3_value to a parameter using a column name.
// An invalid parameter name will cause the call to Step to return an error.
func (stmt *Stmt) SetValue(param string, value Value) {
	stmt.BindValue(stmt.findBindName(param), value)
}

// SetPointer binds a golang value to a parameter using a column name.
func (stmt *Stmt) SetPointer(param string, arg interface{}) {
	stmt.BindPointer(stmt.findBindName(param), arg)
}

// ColumnInt returns a query result value as an int.
//
// Note: this method calls sqlite3_column_int64 and then converts the
// resulting 64-bits to an int.
func (stmt *Stmt) ColumnInt(col int) int {
	return int(stmt.ColumnInt64(col))
}

// ColumnInt32 returns a query result value as an int32.
func (stmt *Stmt) ColumnInt32(col int) int32 {
	return int32(C._sqlite3_column_int(stmt.stmt, C.int(col)))
}

// ColumnInt64 returns a query result value as an int64.
func (stmt *Stmt) ColumnInt64(col int) int64 {
	return int64(C._sqlite3_column_int64(stmt.stmt, C.int(col)))
}

// ColumnBytes reads a query result into buf.
// It reports the number of bytes read.
func (stmt *Stmt) ColumnBytes(col int, buf []byte) int {
	return copy(buf, stmt.columnBytes(col))
}

// ColumnReader creates a byte reader for a query result column.
//
// The reader directly references C-managed memory that stops
// being valid as soon as the statement row resets.
func (stmt *Stmt) ColumnReader(col int) *bytes.Reader {
	// Load the C memory directly into the Reader.
	// There is no exported method that lets it escape.
	return bytes.NewReader(stmt.columnBytes(col))
}

func (stmt *Stmt) columnBytes(col int) []byte {
	p := C._sqlite3_column_blob(stmt.stmt, C.int(col))
	if p == nil {
		return nil
	}
	n := stmt.ColumnLen(col)
	var slice = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(p)), Len: n, Cap: n}))
	return slice
}

// ColumnType returns the datatype code for the initial data
// type of the result column.
func (stmt *Stmt) ColumnType(col int) ColumnType {
	return ColumnType(C._sqlite3_column_type(stmt.stmt, C.int(col)))
}

// ColumnText returns a query result as a string.
func (stmt *Stmt) ColumnText(col int) string {
	n := stmt.ColumnLen(col)
	return C.GoStringN((*C.char)(unsafe.Pointer(C._sqlite3_column_text(stmt.stmt, C.int(col)))), C.int(n))
}

// ColumnFloat returns a query result as a float64.
func (stmt *Stmt) ColumnFloat(col int) float64 {
	return float64(C._sqlite3_column_double(stmt.stmt, C.int(col)))
}

// ColumnValue returns a query result as an sqlite_value.
func (stmt *Stmt) ColumnValue(col int) Value {
	return Value{ptr: C._sqlite3_column_value(stmt.stmt, C.int(col))}
}

// ColumnLen returns the number of bytes in a query result.
func (stmt *Stmt) ColumnLen(col int) int {
	return int(C._sqlite3_column_bytes(stmt.stmt, C.int(col)))
}

func (stmt *Stmt) ColumnDatabaseName(col int) string {
	return C.GoString((*C.char)(unsafe.Pointer(C._sqlite3_column_database_name(stmt.stmt, C.int(col)))))
}

func (stmt *Stmt) ColumnTableName(col int) string {
	return C.GoString((*C.char)(unsafe.Pointer(C._sqlite3_column_table_name(stmt.stmt, C.int(col)))))
}

// ColumnIndex returns the index of the column with the given name.
//
// If there is no column with the given name ColumnIndex returns -1.
func (stmt *Stmt) ColumnIndex(colName string) int {
	col, found := stmt.colNames[colName]
	if !found {
		return -1
	}
	return col
}

// GetInt64 returns a query result value for colName as an int64.
func (stmt *Stmt) GetInt64(colName string) int64 {
	col, found := stmt.colNames[colName]
	if !found {
		return 0
	}
	return stmt.ColumnInt64(col)
}

// GetBytes reads a query result for colName into buf.
// It reports the number of bytes read.
func (stmt *Stmt) GetBytes(colName string, buf []byte) int {
	col, found := stmt.colNames[colName]
	if !found {
		return 0
	}
	return stmt.ColumnBytes(col, buf)
}

// GetReader creates a byte reader for colName.
//
// The reader directly references C-managed memory that stops
// being valid as soon as the statement row resets.
func (stmt *Stmt) GetReader(colName string) *bytes.Reader {
	col, found := stmt.colNames[colName]
	if !found {
		return bytes.NewReader(nil)
	}
	return stmt.ColumnReader(col)
}

// GetText returns a query result value for colName as a string.
func (stmt *Stmt) GetText(colName string) string {
	col, found := stmt.colNames[colName]
	if !found {
		return ""
	}
	return stmt.ColumnText(col)
}

// GetFloat returns a query result value for colName as a float64.
func (stmt *Stmt) GetFloat(colName string) float64 {
	col, found := stmt.colNames[colName]
	if !found {
		return 0
	}
	return stmt.ColumnFloat(col)
}

// GetValue returns a query result value for colName as an sqlite_value.
func (stmt *Stmt) GetValue(colName string) Value {
	col, found := stmt.colNames[colName]
	if !found {
		return Value{}
	}
	return stmt.ColumnValue(col)
}

// GetLen returns the number of bytes in a query result for colName.
func (stmt *Stmt) GetLen(colName string) int {
	col, found := stmt.colNames[colName]
	if !found {
		return 0
	}
	return stmt.ColumnLen(col)
}

// Readonly returns true if this statement is readonly and makes no direct changes to the content of the database file.
// See: https://www.sqlite.org/c3ref/stmt_readonly.html
func (stmt *Stmt) Readonly() bool {
	return C.int(C._sqlite3_stmt_readonly(stmt.stmt)) != 0
}
