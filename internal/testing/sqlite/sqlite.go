// Package sqlite provides a golang package that calls to sqlite3_auto_extension to
// automatically load the extension with every new connection.
// This module is primarily for use with unit tests in the primary module.
package sqlite

// #cgo CFLAGS: -DSQLITE_CORE
//
// #include "../../../sqlite3.h"
//
// // extension function defined in the archive from go.riyazali.net/sqlite
// // the symbol is only available during the final linkage when compiling the binary
// extern int sqlite3_extension_init(sqlite3*, char**, const sqlite3_api_routines*);
import "C"

// register sqlite3_extension_init with sqlite3_auto_extension so that
// the extension is registered with all the database connections
// opened with the sqlite3 library
func init() { C.sqlite3_auto_extension((*[0]byte)(C.sqlite3_extension_init)) }
