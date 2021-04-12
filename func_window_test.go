package sqlite_test

import (
	"database/sql"
	. "go.riyazali.net/sqlite"
	"testing"
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

func (s *Sum) Step(ctx *AggregateContext, values ...Value) {
	if ctx.Data() == nil {
		ctx.SetData(&SumContext{})
	}

	var val = values[0]
	var sumCtx = ctx.Data().(*SumContext)

	if !val.IsNil() {
		sumCtx.count++
		if val.Type() == SQLITE_INTEGER {
			sumCtx.iSum += val.Int64()
		} else {
			sumCtx.approx = true
			sumCtx.rSum += val.Float()
		}
	}
}

func (s *Sum) Final(ctx *AggregateContext) {
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

func (s *Sum) Inverse(ctx *AggregateContext, values ...Value) {
	var val = values[0]
	var sumCtx = ctx.Data().(*SumContext)
	if val.Type() == SQLITE_INTEGER && !sumCtx.approx {
		var v = val.Int64()
		sumCtx.rSum -= float64(v)
		sumCtx.iSum -= v
	} else {
		sumCtx.rSum -= val.Float()
	}
}

func (s *Sum) Value(ctx *AggregateContext) { s.Final(ctx) }

func TestWindowFunction(t *testing.T) {
	var err error

	Register(func(api *ExtensionApi) (ErrorCode, error) {
		if err := api.CreateFunction("sum", &Sum{}); err != nil {
			return SQLITE_ERROR, err
		}
		return SQLITE_OK, nil
	})

	var db *sql.DB
	if db, err = Connect(Memory); err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	t.Run("normal aggregation", func(t *testing.T) {
		var stmt *sql.Stmt
		stmt, err = db.Prepare(`
	WITH RECURSIVE generate_series(value) AS (
	    SELECT 1
	    	UNION ALL
	    SELECT value+1 FROM generate_series
	    	WHERE value+1<=10
	) SELECT SUM(value) FROM generate_series`)

		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()

		var rows *sql.Rows
		if rows, err = stmt.Query(); err != nil {
			t.Fatal(err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Fatal("expected query to return single row")
		}

		var result int
		if err = rows.Scan(&result); err != nil {
			t.Fatal(err)
		}

		if result != 55 {
			t.Fatalf("invalid result: got %q", result)
		}
	})

	t.Run("running sum", func(t *testing.T) {
		var stmt *sql.Stmt
		stmt, err = db.Prepare(`
	WITH RECURSIVE generate_series(value) AS (
	    SELECT 1
	    	UNION ALL
	    SELECT value+1 FROM generate_series
	    	WHERE value+1<=10
	) SELECT SUM(value) OVER(ROWS UNBOUNDED  PRECEDING) AS running_total FROM generate_series`)

		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()

		var rows *sql.Rows
		if rows, err = stmt.Query(); err != nil {
			t.Fatal(err)
		}
		defer rows.Close()

		var series = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		var total = 0

		for i := 0; rows.Next(); i++ {
			total += series[i]

			var j int
			if err = rows.Scan(&j); err != nil {
				t.Fatal(err)
			}

			if total != j {
				t.Fatalf("value mismatch: total(%d) != j(%d)", total, j)
			}
		}
	})
}
