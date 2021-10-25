package main

import (
	"go.riyazali.net/sqlite"
)


const magic = 0xfe

// X(s) is a custom scalar function that returns the same string s,
// but with an added subtype using ResultSubtype. Used with Is_x(s)
// to test subtypes.
type X struct{}

func (m *X) Args() int           { return 1 }
func (m *X) Deterministic() bool { return true }
func (m *X) Apply(ctx *sqlite.Context, values ...sqlite.Value) {
	ctx.ResultText(values[0].Text())
	ctx.ResultSubType(magic)
}

// is_x(s) is a custom scalar function that returns 0 or 1, depending
// if s has the same subtype returned by x(s).
type IsX struct{}

func (m *IsX) Args() int           { return 1 }
func (m *IsX) Deterministic() bool { return true }
func (m *IsX) Apply(ctx *sqlite.Context, values ...sqlite.Value) {
	st := values[0].SubType()
	if st == magic { 
		ctx.ResultInt(1)
	}else {
		ctx.ResultInt(0)
	}
}

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		if err := api.CreateFunction("x", &X{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		if err := api.CreateFunction("is_x", &IsX{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		return sqlite.SQLITE_OK, nil
	})
}

func main() {}
