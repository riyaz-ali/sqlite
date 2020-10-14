package sqlite

import "C"

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
	Connect(args []string, declare func(string) error) (VirtualTable, error)
}

// StatefulModule is one which requires prior state initialization before one can connect to it.
type StatefulModule interface {
	Module

	// Create creates a new instance of a virtual table in response to a CREATE VIRTUAL TABLE statement.
	// It receives a slice of arguments passed to the module and a method to declare the virtual table's schema.
	// Create must call declare or else the operation would fail and error would be returned to sqlite.
	Create(args []string, declare func(string) error) (VirtualTable, error)
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
	// In a rowid virtual table, if the first argument is an SQL NULL, then the implementation must choose
	// a rowid for the newly inserted row. The first argument will be an SQL NULL for a WITHOUT ROWID virtual table,
	// in which case the implementation should take the PRIMARY KEY value from the appropriate column.
	//
	// When doing an Insert on a virtual table that uses ROWID, implementations must return the rowid of
	// the newly inserted row; this will become the value returned by the sqlite3_last_insert_rowid() function.
	// Returning a value for rowid in a WITHOUT ROWID table is a harmless no-op.
	Insert(Value, ...Value) (int64, error)

	// Update updates an existing row identified by the rowid / primary-key given as the first argument.
	Update(Value, ...Value) error

	// UpdateWithChange updates an existing row identified by the rowid / primary-key given as old
	// and replacing it with the new id. The update might also include a list of other columns too.
	// This will occur when an SQL statement updates a rowid, as in the statement:
	//   UPDATE table SET rowid=rowid+1 WHERE ...;
	UpdateWithKeyChange(old, new Value, _ ...Value) error

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
	Constraints []IndexConstraint
	OrderBy     []OrderBy

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
	ConstraintUsage []ConstraintUsage
	IndexNumber     int     // identifier passed on to Cursor.Filter
	IndexString     string  // identifier passed on to Cursor.Filter
	OrderByConsumed bool    // true if output is already ordered
	EstimatedCost   float64 // estimated cost of using this index

	// used only in SQLite 3.8.2 and later
	EstimatedRows int64 // estimated number of rows returned

	// used only in SQLite 3.9.0 and later
	IdxFlags ScanFlag // mask of SQLITE_INDEX_SCAN_* flags
}
