
#ifndef _BRIDGE_H
#define _BRIDGE_H

// This file defines a bridge between Golang and sqlite's c extension api.
// Most of sqlite api function defined in <sqlite3ext.h> are macros that redirect calls
// to an instance of sqlite3_api_routines, called as sqlite3_api.

// As neither macros nor c function pointers work directly in cgo, we need to define a bridge
// to redirect calls from golang to sqlite.

// Most of the methods follow the convention of prefixing the sqlite api function with an underscore.
// The bridge isn't extensive and doesn't cover the whole sqlite api.

#include <sqlite3ext.h>

SQLITE_EXTENSION_INIT3

static int _sqlite3_libversion_number() {
	return sqlite3_libversion_number();
}

void* _sqlite3_result_int(sqlite3_context* ctx, int result) {
	sqlite3_result_int(ctx, result);
}

void* _sqlite3_result_int64(sqlite3_context* ctx, sqlite_int64 result) {
	sqlite3_result_int(ctx, result);
}
#endif // _BRIDGE_H