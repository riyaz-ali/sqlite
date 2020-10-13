package sqlite

// #cgo CFLAGS: -fPIC
//
// #include <stdlib.h>
// #include "sqlite3ext.h"
// #include "bridge/bridge.h"
import "C"

//export go_sqlite3_extension_init
func go_sqlite3_extension_init(db *C.struct_sqlite3, msg **C.char, _ *C.sqlite3_api_routines) (code ErrorCode) {
	var err error
	if code, err = extension(&ExtensionApi{db: db}); err != nil {
		*msg = C.CString(err.Error())
	}
	return
}

// registered singleton extension function
var extension func(*ExtensionApi) (ErrorCode, error)

// Register registers the given fn as the modules extension function.
// The method is not thread-safe and must only be used once, ideally during init(..)
func Register(fn func(*ExtensionApi) (ErrorCode, error)) {
	extension = fn
}

// ExtensionApi wraps the underlying sqlite_api_routines and allows Go code to hook into
// sqlite's extension facility.
type ExtensionApi struct {
	db *C.struct_sqlite3
}

// AutoCommit returns the status of the auto_commit setting
func (ext *ExtensionApi) AutoCommit() bool {
	return int(C._sqlite3_get_autocommit(ext.db)) != 0
}

// Version returns the sqlite3 library version number
func (ext *ExtensionApi) Version() int {
	return int(C._sqlite3_libversion_number())
}

// LimitId is an integer id used to refer to sqlite's limits
type LimitId int

//noinspection GoSnakeCaseUsage
const (
	LIMIT_LENGTH              = LimitId(C.SQLITE_LIMIT_LENGTH)
	LIMIT_SQL_LENGTH          = LimitId(C.SQLITE_LIMIT_SQL_LENGTH)
	LIMIT_COLUMN              = LimitId(C.SQLITE_LIMIT_COLUMN)
	LIMIT_EXPR_DEPTH          = LimitId(C.SQLITE_LIMIT_EXPR_DEPTH)
	LIMIT_COMPOUND_SELECT     = LimitId(C.SQLITE_LIMIT_COMPOUND_SELECT)
	LIMIT_VDBE_OP             = LimitId(C.SQLITE_LIMIT_VDBE_OP)
	LIMIT_FUNCTION_ARG        = LimitId(C.SQLITE_LIMIT_FUNCTION_ARG)
	LIMIT_ATTACHED            = LimitId(C.SQLITE_LIMIT_ATTACHED)
	LIMIT_LIKE_PATTERN_LENGTH = LimitId(C.SQLITE_LIMIT_LIKE_PATTERN_LENGTH)
	LIMIT_VARIABLE_NUMBER     = LimitId(C.SQLITE_LIMIT_VARIABLE_NUMBER)
	LIMIT_TRIGGER_DEPTH       = LimitId(C.SQLITE_LIMIT_TRIGGER_DEPTH)
	LIMIT_WORKER_THREADS      = LimitId(C.SQLITE_LIMIT_WORKER_THREADS)
)

// Limit queries for the limit with given identifier
func (ext *ExtensionApi) Limit(id LimitId) int {
	return int(C._sqlite3_limit(ext.db, C.int(id), C.int(-1)))
}

// SetLimit sets the limit for the given identifier
func (ext *ExtensionApi) SetLimit(id LimitId, val int) int {
	return int(C._sqlite3_limit(ext.db, C.int(id), C.int(val)))
}
