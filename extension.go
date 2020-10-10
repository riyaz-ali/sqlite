package sqlite

// #cgo CFLAGS: -fPIC
//
// #include <stdlib.h>
// #include "sqlite3ext.h"
// #include "bridge.h"
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
