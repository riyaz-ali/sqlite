// +build static

package sqlite

// #cgo CFLAGS: -DSQLITE_CORE -DUSE_LIBSQLITE3
// #cgo LDFLAGS: -lsqlite3
import "C"