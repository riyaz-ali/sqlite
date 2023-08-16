# Recipes 🥘

This file contains some common patterns / recipes useful when building extensions using this library.

## Data Types

`sqlite3` supports only [a handful of types](https://www.sqlite.org/datatype3.html) natively. Other types, such as, [`json`](https://sqlite.org/json1.html), [`date` and `time` types](http://sqlite.org/lang_datefunc.html)
are often implemented using `SQL` functions that interpret the data in specified formats.

### JSON

`sqlite3` stores `json` as `TEXT`, and is marked with the special [subtype](https://www.sqlite.org/c3ref/value_subtype.html) of **`74`**.

To return `json` from your handler, do:

```golang
if value == nil {
  context.ResultNull()
} else if jb, err := json.Marshal(value); err != nil {
  ctx.ResultError(err)
} else {
  context.ResultText(string(b))
  context.ResultSubType(74) // mark with subtype 74 so other sqlite3 functions interprets it as json
}
```

### Date and Time

`sqlite` doesn't have date data types, but is stored as [specific timestamps formats](https://www.sqlite.org/lang_datefunc.html).
To convert Go's `time.Time` and `time.Date` objects, do:

```golang
// timestamp layout to match SQLite's 'datetime()' format, ISO8601 subset
const sqliteDatetimeFormat = "2006-01-02 15:04:05.999"
const sqliteDateFormat = "2006-01-02"

ctx.ResultText(time.Now().Format(sqliteDatetimeFormat))
ctx.ResultText(time.Date(2000, 1, 1).Format(sqliteDateFormat))
```