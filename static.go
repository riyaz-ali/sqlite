//go:build static
// +build static

package sqlite

// #cgo CFLAGS: -DSQLITE_CORE
import "C"
