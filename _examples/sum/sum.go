package main

import (
	"go.riyazali.net/sqlite"
)

// Sum implements a window functions (that also doubles up as a normal aggregate function)
// It follows the spec and description defined at https://sqlite.org/lang_aggfunc.html#sumunc
// The source of this file is adapted from https://sqlite.org/src/file?name=src/func.c
type Sum struct{}

func (s *Sum) Args() int           { return 1 }
func (s *Sum) Deterministic() bool { return true }

type SumContext struct {
	rSum   float64
	iSum   int64
	count  int64
	approx bool
}

func (s *Sum) Step(ctx *sqlite.AggregateContext, values ...sqlite.Value) {
	if ctx.Data() == nil {
		ctx.SetData(&SumContext{})
	}

	var val = values[0]
	var sumCtx = ctx.Data().(*SumContext)

	if !val.IsNil() {
		sumCtx.count++
		if val.Type() == sqlite.SQLITE_INTEGER {
			sumCtx.iSum += val.Int64()
		} else {
			sumCtx.approx = true
			sumCtx.rSum += val.Float()
		}
	}
}

func (s *Sum) Final(ctx *sqlite.AggregateContext) {
	if ctx.Data() != nil {
		var sumCtx = ctx.Data().(*SumContext)
		if sumCtx.count > 0 {
			if sumCtx.approx {
				ctx.ResultFloat(sumCtx.rSum)
			} else {
				ctx.ResultInt64(sumCtx.iSum)
			}
		}
	}
}

func (s *Sum) Inverse(ctx *sqlite.AggregateContext, values ...sqlite.Value) {
	var val = values[0]
	var sumCtx = ctx.Data().(*SumContext)
	if val.Type() == sqlite.SQLITE_INTEGER && !sumCtx.approx {
		var v = val.Int64()
		sumCtx.rSum -= float64(v)
		sumCtx.iSum -= v
	} else {
		sumCtx.rSum -= val.Float()
	}
}

func (s *Sum) Value(ctx *sqlite.AggregateContext) {
	s.Final(ctx)
}

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		if err := api.CreateFunction("sum", &Sum{}); err != nil {
			return sqlite.SQLITE_ERROR, err
		}
		return sqlite.SQLITE_OK, nil
	})
}

func main() {}
