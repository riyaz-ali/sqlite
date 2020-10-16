package main

import (
	"go.riyazali.net/sqlite"
	"strings"
)

// Upper implements a custom Upper(...) scalar sql function
type Upper struct{}

func (m *Upper) Args() int           { return 1 }
func (m *Upper) Deterministic() bool { return true }
func (m *Upper) Apply(ctx *sqlite.Context, values ...sqlite.Value) {
	ctx.ResultText(strings.ToUpper(values[0].Text()))
}

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		if err := api.CreateFunction("upper", &Upper{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		return sqlite.SQLITE_OK, nil
	})
}

func main() {}
