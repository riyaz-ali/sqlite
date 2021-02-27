package sqlite

// #include <stdlib.h>
// #include <string.h>
// #include "sqlite3.h"
// #include "bridge/bridge.h"
//
// extern int x_create_tramp(sqlite3*, void*, int, char**, sqlite3_vtab**, char**);
// extern int x_connect_tramp(sqlite3*, void*, int, char**, sqlite3_vtab**, char**);
// extern int x_best_index_tramp(sqlite3_vtab*, sqlite3_index_info*);
// extern int x_disconnect_tramp(sqlite3_vtab*);
// extern int x_destroy_tramp(sqlite3_vtab*);
// extern int x_open_tramp(sqlite3_vtab*, sqlite3_vtab_cursor**);
// extern int x_close_tramp(sqlite3_vtab_cursor*);
// extern int x_filter_tramp(sqlite3_vtab_cursor*, int, char*, int, sqlite3_value**);
// extern int x_next_tramp(sqlite3_vtab_cursor*);
// extern int x_eof_tramp(sqlite3_vtab_cursor*);
// extern int x_column_tramp(sqlite3_vtab_cursor*, sqlite3_context*, int);
// extern int x_rowid_tramp(sqlite3_vtab_cursor*, sqlite3_int64*);
// extern int x_update_tramp(sqlite3_vtab*, int, sqlite3_value**, sqlite3_int64*);
// extern int x_begin_tramp(sqlite3_vtab*);
// extern int x_sync_tramp(sqlite3_vtab*);
// extern int x_commit_tramp(sqlite3_vtab*);
// extern int x_rollback_tramp(sqlite3_vtab*);
//
// extern void module_destroy(void*);
//
// static sqlite3_module* _allocate_sqlite3_module() {
//   sqlite3_module* module = (sqlite3_module*) _sqlite3_malloc(sizeof(sqlite3_module));
//   memset(module, 0, sizeof(sqlite3_module));
//   return module;
// }
//
// typedef struct go_virtual_table go_virtual_table;
// struct go_virtual_table {
//   sqlite3_vtab base;  // base class - must be first
//   void *impl;  // pointer to go virtual table implementation
// };
//
// static int _allocate_virtual_table(sqlite3_vtab **out, void *impl){
//   go_virtual_table* table = (go_virtual_table*) _sqlite3_malloc(sizeof(go_virtual_table));
//   if (!table) {
//     return SQLITE_NOMEM;
//   }
//   memset(table, 0, sizeof(go_virtual_table));
//	 table->impl = impl;
//   *out = (sqlite3_vtab*) table;
//   return SQLITE_OK;
// }
//
// typedef struct go_virtual_cursor go_virtual_cursor;
// struct go_virtual_cursor {
//   sqlite3_vtab_cursor base;  // base class - must be first
//   void *impl;  // pointer to go virtual cursor implementation
// };
//
// static int _allocate_virtual_cursor(sqlite3_vtab_cursor **out, void *impl){
//   go_virtual_cursor* cursor = (go_virtual_cursor*) _sqlite3_malloc(sizeof(go_virtual_cursor));
//   if (!cursor) {
//     return SQLITE_NOMEM;
//   }
//   memset(cursor, 0, sizeof(go_virtual_cursor));
//	 cursor->impl = impl;
//   *out = (sqlite3_vtab_cursor*) cursor;
//   return SQLITE_OK;
// }
//
import "C"

import (
	"bytes"
	"errors"
	"github.com/mattn/go-pointer"
	"reflect"
	"strings"
	"unsafe"
)

// Module corresponds to an sqlite3_module and defines a module object used to implement a virtual table.
// The Module API is adapted to feel more Go-like and so, overall, is split into various sub-types
// all of which the implementer must provide in order to satisfy a sqlite_module interface.
type Module interface {
	// Connect connects to an existing instance and establishes a new connection to an existing virtual table.
	// It receives a slice of arguments passed to the module and a method to declare the virtual table's schema.
	// Connect must call declare or else the operation would fail and error would be returned to sqlite.
	//
	// If a module only declares a Connect method than it's an eponymous module (by default) and can either be
	// invoked directly or be used in a CREATE VIRTUAL TABLE statement. To make eponymous-only use EponymousOnly(true).
	// see: https://www.sqlite.org/vtab.html for details about normal, eponymous and eponymous-only modules.
	Connect(_ *Conn, args []string, declare func(string) error) (VirtualTable, error)
}

// StatefulModule is one which requires prior state initialization before one can connect to it.
type StatefulModule interface {
	Module

	// Create creates a new instance of a virtual table in response to a CREATE VIRTUAL TABLE statement.
	// It receives a slice of arguments passed to the module and a method to declare the virtual table's schema.
	// Create must call declare or else the operation would fail and error would be returned to sqlite.
	Create(_ *Conn, args []string, declare func(string) error) (VirtualTable, error)
}

// VirtualTable corresponds to an sqlite3_vtab and defines a virtual table object used to implement a virtual table.
// By default implementations of this interface are considered read-only. Implement WriteableVirtualTable and use
// ReadOnly(false) option to enable read-write support.
type VirtualTable interface {
	// BestIndex is used to determine whether an index is available and can be used to optimise query for the table.
	// SQLite uses the BestIndex method of a virtual table module to determine the best way to access the virtual table.
	BestIndex(*IndexInfoInput) (*IndexInfoOutput, error)

	// Open creates a new cursor used for accessing (read and/or writing) a virtual table.
	// When initially opened, the cursor is in an undefined state. The SQLite core will invoke the
	// Filter method on the cursor prior to any attempt to position or read from the cursor.
	// A virtual table implementation must be able to support an arbitrary number of simultaneously open cursors.
	Open() (VirtualCursor, error)

	// Disconnect releases a connection to a virtual table.
	// Only the virtual table connection is destroyed.
	// The virtual table is not destroyed and any backing store associated with the virtual table persists.
	Disconnect() error

	// Destroy releases a connection to a virtual table, just like the Disconnect method,
	// and it also destroys the underlying table implementation.
	Destroy() error
}

// WriteableVirtualTable is one that supports INSERT, UPDATE and/or DELETE operations.
//
// There might be one or more cursor objects open and in use on the virtual table instance
// and perhaps even on the row of the virtual table when the write methods are invoked.
// The implementation must be prepared for attempts to delete or modify rows of the table
// out from other existing cursors. If the virtual table cannot accommodate such changes,
// the methods must return an error code.
type WriteableVirtualTable interface {
	VirtualTable

	// Insert inserts a new row reading column values from the passed in list of values.
	// In a rowid virtual table, the implementation must choose a rowid for the newly inserted row.
	// For a WITHOUT ROWID virtual table, implementation should take the PRIMARY KEY value from the appropriate column.
	//
	// When doing an Insert on a virtual table that uses ROWID, implementations must return the rowid of
	// the newly inserted row; this will become the value returned by the sqlite3_last_insert_rowid() function.
	// Returning a value for rowid in a WITHOUT ROWID table is a harmless no-op.
	Insert(...Value) (int64, error)

	// Update updates an existing row identified by the rowid / primary-key given as the first argument.
	Update(Value, ...Value) error

	// Replace replaces an existing row identified by the rowid / primary-key given as old
	// and replacing it with the new id. The update might also include a list of other columns too.
	// This will occur when an SQL statement updates a rowid, as in the statement:
	//   UPDATE table SET rowid=rowid+1 WHERE ...;
	Replace(old, new Value, _ ...Value) error

	// Delete deletes the row identified the rowid / primary-key in the given value.
	Delete(Value) error
}

// Transactional is an optional interface that VirtualTable implementations can implement to enable support
// for atomic transactions.
type Transactional interface {
	VirtualTable

	// Begin begins a transaction on a virtual table.
	// This method is always followed by one call to either the Commit or Rollback method.
	// Virtual table transactions do not nest, so the Begin method will not be invoked more
	// than once on a single virtual table without an intervening call to either Commit or Rollback.
	Begin() error

	// Commit is invoked to commit the current virtual table transaction.
	Commit() error

	// Rollback is invoked to rollback the current virtual table transaction.
	Rollback() error
}

// TwoPhaseCommitter is an optional interface that VirtualTable implementations (which also implements Transactional)
// can implement to enable support for two-phased commits.
type TwoPhaseCommitter interface {
	Transactional

	// Sync signals the start of a two-phase commit on a virtual table.
	// It is only invoked after call to the Begin method and prior to an Commit or Rollback.
	// In order to implement two-phase commit, the Sync method on all virtual tables is invoked
	// prior to invoking the Commit method on any virtual table. If any of the Sync methods fail,
	// the entire transaction is rolled back.
	Sync() error
}

// OverloadableVirtualTable is an optional interface the VirtualTable implementations can implement
// to allow them an opportunity to overload functions, replacing them with optimised implementations.
// For more details and implementation notes, please refer to official
// documentation at https://www.sqlite.org/vtab.html#the_xfindfunction_method
type OverloadableVirtualTable interface {
	VirtualTable

	// FindFunction is called during sqlite3_prepare() to give the virtual table
	// implementation an opportunity to overload functions.
	// When a function uses a column from a virtual table as its first argument,
	// this method is called to see if the virtual table would like to overload the function.
	// The method receives the SQL function name and it's argument count as arguments.
	// Please refer to official SQLite documentation to find more about the acceptable return values.
	FindFunction(string, int) (int, func(*Context, ...Value))
}

// VirtualCursor corresponds to an sqlite3_vtab_cursor.
// The cursor represents a pointer to a specific row of a virtual table
type VirtualCursor interface {
	// Filter begins a search of a virtual table.
	// It receives the IndexNumber and IndexString returned by BestIndex method previously.
	// The BestIndex function may have also requested the values of certain expressions using
	// the ConstraintUsage[].ArgvIndex values of the IndexInfoOutput. Those values are passed
	// to Filter as a list of Value objects.
	Filter(int, string, ...Value) error

	// Next advances a virtual table cursor to the next row of a result set initiated by Filter.
	// If the cursor is already pointing at the last row when this routine is called,
	// then the cursor no longer points to valid data and a subsequent call to the Eof method must return true.
	// If the cursor is successfully advanced to another row of content, then subsequent calls to xEof must return false.
	Next() error

	// Rowid returns the rowid of row that the virtual table cursor is currently pointing at.
	Rowid() (int64, error)

	// Column is invoked by SQLite core in order to find the value for the N-th column of the current row.
	// N is zero-based so the first column is numbered 0. Column may return its result back using the various ResultX
	// methods defined on the Context argument. If the implementation calls none of those functions,
	// then the value of the column defaults to an SQL NULL.
	Column(*Context, int) error

	// Eof returns false if the specified cursor currently points to a valid row of data,
	// or true otherwise. This method is called by the SQL engine immediately after each Filter and Next invocation.
	Eof() bool

	// Close closes a cursor previously opened by Open.
	// The SQLite core will always call Close once for each cursor opened using Open.
	Close() error
}

// ConstraintOp op-code passed as input in BestIndex
type ConstraintOp C.int

//noinspection GoSnakeCaseUsage
const (
	INDEX_CONSTRAINT_EQ        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_EQ)
	INDEX_CONSTRAINT_GT        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_GT)
	INDEX_CONSTRAINT_LE        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_LE)
	INDEX_CONSTRAINT_LT        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_LT)
	INDEX_CONSTRAINT_GE        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_GE)
	INDEX_CONSTRAINT_MATCH     = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_MATCH)
	INDEX_CONSTRAINT_LIKE      = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_LIKE)
	INDEX_CONSTRAINT_GLOB      = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_GLOB)
	INDEX_CONSTRAINT_REGEXP    = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_REGEXP)
	INDEX_CONSTRAINT_NE        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_NE)
	INDEX_CONSTRAINT_ISNOT     = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_ISNOT)
	INDEX_CONSTRAINT_ISNOTNULL = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_ISNOTNULL)
	INDEX_CONSTRAINT_ISNULL    = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_ISNULL)
	INDEX_CONSTRAINT_IS        = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_IS)
	INDEX_CONSTRAINT_FUNCTION  = ConstraintOp(C.SQLITE_INDEX_CONSTRAINT_FUNCTION)
)

// ScanFlags masking bits used by virtual table implementations to set the IndexInfoOutput.IdxFlags field
type ScanFlag int

//noinspection GoSnakeCaseUsage
const (
	INDEX_SCAN_UNIQUE = ScanFlag(C.SQLITE_INDEX_SCAN_UNIQUE) // scan visits at most 1 row
)

type IndexConstraint struct {
	ColumnIndex int          // column constrained .. -1 for rowid
	Op          ConstraintOp // constraint operator
	Usable      bool         // true if this constraint is usable
}

type OrderBy struct {
	ColumnIndex int  // column index
	Desc        bool // descending or ascending?
}

// IndexInfoInput is the input provided to the BestIndex method
// see: https://www.sqlite.org/vtab.html for more details
type IndexInfoInput struct {
	Constraints []*IndexConstraint
	OrderBy     []*OrderBy

	//  available only in SQLite 3.10.0 and later
	ColUsed int64 // Mask of columns used by statement
}

// ConstraintUsage provides details about whether a constraint provided in IndexInfoInput
// was / can be utilised or not by the index.
type ConstraintUsage struct {
	ArgvIndex int
	Omit      bool
}

// IndexInfoOutput is the output expected from BestIndex method
type IndexInfoOutput struct {
	ConstraintUsage []*ConstraintUsage
	IndexNumber     int     // identifier passed on to Cursor.Filter
	IndexString     string  // identifier passed on to Cursor.Filter
	OrderByConsumed bool    // true if output is already ordered
	EstimatedCost   float64 // estimated cost of using this index

	// used only in SQLite 3.8.2 and later
	EstimatedRows int64 // estimated number of rows returned

	// used only in SQLite 3.9.0 and later
	IdxFlags ScanFlag // mask of SQLITE_INDEX_SCAN_* flags
}

// ModuleOptions represents the various different module options that affect the module's registration process
type ModuleOptions struct {
	EponymousOnly  bool // CREATE VIRTUAL TABLE is prohibited for eponymous-only virtual tables
	ReadOnly       bool // Insert / Update / Delete is not allowed on read-only tables
	Transactional  bool // Transactional must be set if the table implements the optional Transactional interface
	TwoPhaseCommit bool // TwoPhaseCommit must be set if the table supports two-phase commits (implies Transactional)
	Overloadable   bool // Overloadable must be set if the table supports overloading default functions / operations
}

// CreateModule creates a named virtual table module with the given name and module as implementation.
func (ext *ExtensionApi) CreateModule(name string, module Module, opts ...func(*ModuleOptions)) error {
	var cname = C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var opt = &ModuleOptions{ReadOnly: true} // false is default for rest of the fields
	for _, f := range opts {
		f(opt)
	}

	if _, stateful := module.(StatefulModule); opt.EponymousOnly && stateful {
		return errors.New("stateful module cannot be eponymous-only")
	}

	// the sqlite3_module interface
	var xCreate, xConnect *[0]byte                             // sqlite3_module routines
	var xBestIndex, xOpen, xDisconnect, xDestroy *[0]byte      // sqlite3_vtab mandatory routines
	var xUpdate *[0]byte                                       // sqlite3_vtab writeable routine
	var xBegin, xCommit, xRollback *[0]byte                    // sqlite3_vtab transactional routines
	var xSync *[0]byte                                         // sqlite3_vtab two-phase commit routine
	var xFindFunction *[0]byte                                 // sqlite3_vtab overload-able routine
	var xFilter, xNext, xRowid, xColumn, xEof, xClose *[0]byte // sqlite3_vtab cursor routines

	xConnect = (*[0]byte)(C.x_connect_tramp)
	if !opt.EponymousOnly {
		xCreate = xConnect // for eponymous tables, xCreate and xConnect must point to same routine, else it's set to nil
	} else if _, stateful := module.(StatefulModule); stateful {
		xCreate = (*[0]byte)(C.x_create_tramp)
	}

	xBestIndex = (*[0]byte)(C.x_best_index_tramp)
	xOpen = (*[0]byte)(C.x_open_tramp)
	xDisconnect = (*[0]byte)(C.x_disconnect_tramp)
	xDestroy = (*[0]byte)(C.x_destroy_tramp)

	if !opt.ReadOnly {
		xUpdate = (*[0]byte)(C.x_update_tramp)
	}

	if opt.Transactional {
		xBegin = (*[0]byte)(C.x_begin_tramp)
		xCommit = (*[0]byte)(C.x_commit_tramp)
		xRollback = (*[0]byte)(C.x_rollback_tramp)

		if opt.TwoPhaseCommit {
			xSync = (*[0]byte)(C.x_sync_tramp)
		}
	}

	if opt.Overloadable {
		// TODO: implement x_find_function_tramp
	}

	xFilter = (*[0]byte)(C.x_filter_tramp)
	xNext = (*[0]byte)(C.x_next_tramp)
	xRowid = (*[0]byte)(C.x_rowid_tramp)
	xColumn = (*[0]byte)(C.x_column_tramp)
	xEof = (*[0]byte)(C.x_eof_tramp)
	xClose = (*[0]byte)(C.x_close_tramp)

	var sqliteModule = C._allocate_sqlite3_module()
	sqliteModule.iVersion = 0
	sqliteModule.xCreate = xCreate
	sqliteModule.xConnect = xConnect
	sqliteModule.xBestIndex = xBestIndex
	sqliteModule.xDisconnect = xDisconnect
	sqliteModule.xDestroy = xDestroy
	sqliteModule.xOpen = xOpen
	sqliteModule.xClose = xClose
	sqliteModule.xFilter = xFilter
	sqliteModule.xNext = xNext
	sqliteModule.xEof = xEof
	sqliteModule.xColumn = xColumn
	sqliteModule.xRowid = xRowid
	sqliteModule.xUpdate = xUpdate
	sqliteModule.xBegin = xBegin
	sqliteModule.xSync = xSync
	sqliteModule.xCommit = xCommit
	sqliteModule.xRollback = xRollback
	sqliteModule.xFindFunction = xFindFunction

	var res = C._sqlite3_create_module_v2(ext.db, cname, sqliteModule, pointer.Save(module), (*[0]byte)(C.module_destroy))

	if ErrorCode(res) == SQLITE_OK {
		return nil
	}
	return ErrorCode(res)
}

// options for CreateModule ...

// @formatter:off
func EponymousOnly(b bool) func(*ModuleOptions)  { return func(m *ModuleOptions) { m.EponymousOnly = b } }
func ReadOnly(b bool) func(*ModuleOptions)       { return func(m *ModuleOptions) { m.ReadOnly = b } }
func Transaction(b bool) func(*ModuleOptions)    { return func(m *ModuleOptions) { m.Transactional = b } }
func TwoPhaseCommit(b bool) func(*ModuleOptions) { return func(m *ModuleOptions) { m.TwoPhaseCommit = b } }
func Overloadable(b bool) func(*ModuleOptions)   { return func(m *ModuleOptions) { m.Overloadable = b } }
// @formatter:on

// TRAMPOLINES AHEAD!!

// shared code used by xCreate & xConnect tramps
func create_connect_shared(db *C.sqlite3, fn func(_ *Conn, args []string, declare func(string) error) (VirtualTable, error), argc C.int, argv **C.char, vtab **C.sqlite3_vtab, pzErr **C.char) C.int {
	var err error

	// helper function passed to Create/Connect to invoke sqlite3_declare_vtab
	var declare = func(sql string) error {
		var csql = C.CString(sql)
		defer C.free(unsafe.Pointer(csql))
		if res := C._sqlite3_declare_vtab(db, csql); res != C.SQLITE_OK {
			return ErrorCode(res)
		}
		return nil
	}

	var args = make([]string, argc)
	{ // convert **C.char into []string
		var slice = *(*[]*C.char)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(argv)), Len: int(argc), Cap: int(argc)}))
		for i, s := range slice {
			args[i] = C.GoString(s)
		}
	}

	var table VirtualTable
	if table, err = fn(wrap(db), args, declare); err != nil && err != SQLITE_OK {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		*pzErr = _allocate_string(err.Error())
		return C.int(SQLITE_ERROR)
	}

	return C._allocate_virtual_table(vtab, pointer.Save(table))
}

//export x_create_tramp
func x_create_tramp(db *C.sqlite3, pAux unsafe.Pointer, argc C.int, argv **C.char, vtab **C.sqlite3_vtab, pzErr **C.char) C.int {
	var module = pointer.Restore(pAux).(StatefulModule)
	return create_connect_shared(db, module.Create, argc, argv, vtab, pzErr)
}

//export x_connect_tramp
func x_connect_tramp(db *C.sqlite3, pAux unsafe.Pointer, argc C.int, argv **C.char, vtab **C.sqlite3_vtab, pzErr **C.char) C.int {
	var module = pointer.Restore(pAux).(Module)
	return create_connect_shared(db, module.Connect, argc, argv, vtab, pzErr)
}

//export x_best_index_tramp
func x_best_index_tramp(tab *C.sqlite3_vtab, indexInfo *C.sqlite3_index_info) C.int {
	var version = int(C._sqlite3_libversion_number())
	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(VirtualTable)

	var constraints []*IndexConstraint
	{
		var slice = *(*[]C.struct_sqlite3_index_constraint)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(indexInfo.aConstraint)),
			Len:  int(indexInfo.nConstraint),
			Cap:  int(indexInfo.nConstraint),
		}))
		for _, cons := range slice {
			constraints = append(constraints,
				&IndexConstraint{ColumnIndex: int(cons.iColumn), Op: ConstraintOp(cons.op), Usable: int(cons.usable) != 0})
		}
	}

	var orderBys []*OrderBy
	{
		var slice = *(*[]C.struct_sqlite3_index_orderby)(unsafe.Pointer(&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(indexInfo.aOrderBy)),
			Len:  int(indexInfo.nOrderBy),
			Cap:  int(indexInfo.nOrderBy),
		}))
		for _, ob := range slice {
			orderBys = append(orderBys, &OrderBy{ColumnIndex: int(ob.iColumn), Desc: int(ob.desc) == 1})
		}
	}

	var input = &IndexInfoInput{Constraints: constraints, OrderBy: orderBys}
	if version >= 3010000 {
		input.ColUsed = int64(indexInfo.colUsed)
	}

	output, err := table.BestIndex(input)
	if err != nil && err != SQLITE_OK {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	} else if output == nil {
		return C.int(SQLITE_ERROR)
	}

	// Get a pointer to constraint_usage struct so we can update in place.
	// indexInfo.aConstraintUsage comes pre-allocated by SQLite core
	var usage = *(*[]C.struct_sqlite3_index_constraint_usage)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(indexInfo.aConstraintUsage)),
		Len:  int(indexInfo.nConstraint),
		Cap:  int(indexInfo.nConstraint),
	}))

	for i, c := range output.ConstraintUsage {
		if c != nil { // usage must be ordered .. and a list value might be nil so we better check this
			usage[i].argvIndex = C.int(c.ArgvIndex)
			if c.Omit {
				usage[i].omit = C.uchar(1)
			}
		}
	}

	indexInfo.idxNum = C.int(output.IndexNumber)
	indexInfo.idxStr = _allocate_string(output.IndexString)
	indexInfo.needToFreeIdxStr = C.int(1)
	if output.OrderByConsumed {
		indexInfo.orderByConsumed = C.int(1)
	}
	indexInfo.estimatedCost = C.double(output.EstimatedCost)
	if version >= 3008002 {
		indexInfo.estimatedRows = C.sqlite3_int64(output.EstimatedRows)
	}
	if version >= 3009000 {
		indexInfo.idxFlags = C.int(output.IdxFlags)
	}

	return C.int(SQLITE_OK)
}

//export x_disconnect_tramp
func x_disconnect_tramp(tab *C.sqlite3_vtab) C.int {
	var x = unsafe.Pointer(tab)
	defer func() { pointer.Unref((*C.go_virtual_table)(x).impl); C._sqlite3_free(x) }()

	var table = pointer.Restore((*C.go_virtual_table)(x).impl).(VirtualTable)
	if err := table.Disconnect(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_destroy_tramp
func x_destroy_tramp(tab *C.sqlite3_vtab) C.int {
	var x = unsafe.Pointer(tab)
	defer func() { pointer.Unref((*C.go_virtual_table)(x).impl); C._sqlite3_free(x) }()

	var table = pointer.Restore((*C.go_virtual_table)(x).impl).(VirtualTable)
	if err := table.Destroy(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_open_tramp
func x_open_tramp(tab *C.sqlite3_vtab, cur **C.sqlite3_vtab_cursor) C.int {
	var err error

	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(VirtualTable)
	var cursor VirtualCursor
	if cursor, err = table.Open(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}

	return C._allocate_virtual_cursor(cur, pointer.Save(cursor))
}

//export x_update_tramp
func x_update_tramp(tab *C.sqlite3_vtab, c C.int, v **C.sqlite3_value, rowid *C.sqlite3_int64) C.int {
	var equivalent = func(typ ColumnType, v0, v1 Value) bool {
		switch typ {
		case SQLITE_INTEGER:
			return v0.Int() == v1.Int() || v0.Int64() == v1.Int64()
		case SQLITE_FLOAT:
			return v0.Float() == v1.Float()
		case SQLITE_TEXT:
			return strings.Compare(v0.Text(), v1.Text()) == 0
		case SQLITE_BLOB:
			return bytes.Equal(v0.Blob(), v1.Blob())
		}
		return false
	}

	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(WriteableVirtualTable)
	argc, argv := int(c), toValues(c, v)
	var err error

	if argc == 1 && argv[0].Type() != SQLITE_NULL {
		err = table.Delete(argv[0])
	} else {
		if argv[0].Type() == SQLITE_NULL {
			var id int64
			if id, err = table.Insert(argv[2:]...); err == nil {
				*rowid = C.sqlite3_int64(id) // is a harmless no-op if it's a WITHOUT ROWID table
			}
		} else if equivalent(argv[0].Type(), argv[0], argv[1]) {
			err = table.Update(argv[0], argv[2:]...)
		} else {
			err = table.Replace(argv[0], argv[1], argv[2:]...)
		}
	}

	if err != nil && err != SQLITE_OK {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}

	return C.int(SQLITE_OK)
}

//export x_close_tramp
func x_close_tramp(cur *C.sqlite3_vtab_cursor) C.int {
	var x = unsafe.Pointer(cur)
	defer func() { pointer.Unref((*C.go_virtual_cursor)(x).impl); C._sqlite3_free(x) }()

	var cursor = pointer.Restore((*C.go_virtual_cursor)(x).impl).(VirtualCursor)
	if err := cursor.Close(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(cur.pVtab, err)
	}

	return C.int(SQLITE_OK)
}

//export x_filter_tramp
func x_filter_tramp(cur *C.sqlite3_vtab_cursor, idxNum C.int, idxStr *C.char, argc C.int, valarray **C.sqlite3_value) C.int {
	var cursor = pointer.Restore(((*C.go_virtual_cursor)(unsafe.Pointer(cur))).impl).(VirtualCursor)
	var str = C.GoString(idxStr)
	if err := cursor.Filter(int(idxNum), str, toValues(argc, valarray)...); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(cur.pVtab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_next_tramp
func x_next_tramp(cur *C.sqlite3_vtab_cursor) C.int {
	var cursor = pointer.Restore(((*C.go_virtual_cursor)(unsafe.Pointer(cur))).impl).(VirtualCursor)
	if err := cursor.Next(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(cur.pVtab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_eof_tramp
func x_eof_tramp(cur *C.sqlite3_vtab_cursor) C.int {
	var cursor = pointer.Restore(((*C.go_virtual_cursor)(unsafe.Pointer(cur))).impl).(VirtualCursor)
	if cursor.Eof() {
		return C.int(1)
	}
	return C.int(0)
}

//export x_column_tramp
func x_column_tramp(cur *C.sqlite3_vtab_cursor, c *C.sqlite3_context, idx C.int) C.int {
	var cursor = pointer.Restore(((*C.go_virtual_cursor)(unsafe.Pointer(cur))).impl).(VirtualCursor)
	var ctx = &Context{ptr: c}
	if err := cursor.Column(ctx, int(idx)); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			ctx.ResultText(ec.String())
			return C.int(ec)
		}
		ctx.ResultText(err.Error())
		return C.int(SQLITE_ERROR)
	}
	return C.int(SQLITE_OK)
}

//export x_rowid_tramp
func x_rowid_tramp(cur *C.sqlite3_vtab_cursor, rowid *C.sqlite3_int64) C.int {
	var cursor = pointer.Restore(((*C.go_virtual_cursor)(unsafe.Pointer(cur))).impl).(VirtualCursor)
	if id, err := cursor.Rowid(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(cur.pVtab, err)
	} else {
		*rowid = C.sqlite3_int64(id)
	}
	return C.int(SQLITE_OK)
}

//export x_begin_tramp
func x_begin_tramp(tab *C.sqlite3_vtab) C.int {
	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(Transactional)
	if err := table.Begin(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_sync_tramp
func x_sync_tramp(tab *C.sqlite3_vtab) C.int {
	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(TwoPhaseCommitter)
	if err := table.Sync(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_commit_tramp
func x_commit_tramp(tab *C.sqlite3_vtab) C.int {
	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(Transactional)
	if err := table.Commit(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}
	return C.int(SQLITE_OK)
}

//export x_rollback_tramp
func x_rollback_tramp(tab *C.sqlite3_vtab) C.int {
	var table = pointer.Restore(((*C.go_virtual_table)(unsafe.Pointer(tab))).impl).(Transactional)
	if err := table.Rollback(); err != nil {
		if ec, ok := err.(ErrorCode); ok {
			return C.int(ec)
		}
		return set_error_message(tab, err)
	}
	return C.int(SQLITE_OK)
}

//export module_destroy
func module_destroy(pAux unsafe.Pointer) { pointer.Unref(pAux) }

// helper to set the error message field for the cursor
func set_error_message(vtab *C.sqlite3_vtab, err error) C.int {
	if vtab.zErrMsg != nil {
		C._sqlite3_free(unsafe.Pointer(vtab.zErrMsg))
	}

	vtab.zErrMsg = _allocate_string(err.Error())
	return C.int(SQLITE_ERROR)
}

// helper to allocate a string for error using sqlite3_malloc
func _allocate_string(msg string) *C.char {
	var l = len(msg)+1
	var dst = C._sqlite3_malloc(C.int(l))

	// buf is go representation of dst, so that we can do copy(buf, ...)
	var buf = *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(unsafe.Pointer(dst)), Len: l, Cap: l}))
	copy(buf, msg)
	buf[l-1] = 0 // null-terminate the resulting string

	return (*C.char)(dst)
}
