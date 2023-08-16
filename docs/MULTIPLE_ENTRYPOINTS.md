# Multiple Entrtypoints

The [`sqlite3_load_extension()`](https://www.sqlite.org/c3ref/load_extension.html) interface (used to load extension) supports `zProc` parameter.

`zProc` specifies the entry-point routine to invoke when the extension is loaded. This allows us to ship multiple _variants_ of the extension
as a single file, allowing the user to pick at runtime (a use-case described [here](https://github.com/riyaz-ali/sqlite/issues/9#issue-1338323275)).

Supporting this entirely in the scope of the library isn't feasible, and so, the dependent package needs to add some boilerplate `c` source file,
and enable `cgo` for package compilation (since we're anyways relying on `cgo` for `sqlite3` this shouldn't be a problem).

A gist demonstrating the approach is available at https://gist.github.com/riyaz-ali/53959b1b7addb107e50340359e553ddd