# SQLite Extensions

[![Go v1.14](https://img.shields.io/badge/v1.14-blue.svg?labelColor=a8bfc0&color=5692c7&logoColor=fff&style=for-the-badge&logo=Go)](https://golang.org/doc/go1.14)
[![CGO](https://img.shields.io/badge/requires_cgo-blue.svg?labelColor=a8bfc0&color=5692c7&logoColor=fff&style=for-the-badge&logo=Go)](https://golang.org/doc/go1.14)
[![Godoc](https://img.shields.io/badge/godoc-reference-blue.svg?labelColor=a8bfc0&color=5692c7&logoColor=fff&style=for-the-badge)](https://pkg.go.dev/go.riyazali.net/sqlite)

## Overview

**`sqlite`** package provides a low-level interface that allows you to build [**`sqlite`** extensions](https://www.sqlite.org/loadext.html) that [_can be loaded dynamically at runtime_](https://www.sqlite.org/loadext.html#loading_an_extension)
or [_linked statically at build-time_](https://www.sqlite.org/loadext.html#statically_linking_a_run_time_loadable_extension) <sup>(experimental)</sup>

## Installation

This package can be installed with `go get` as:

```shell
$ go get -u go.riyazali.net/sqlite
```

**`sqlite`** is a `cgo` package and requires a working `c` compiler.

## Usage

To build an `sqlite` extension, you need to build your project with [`-buildmode=c-shared`](https://golang.org/cmd/go/#hdr-Build_modes). That would emit
a **`.so`** file (or **`.dll`** on windows), which you can then [_load into `sqlite`_](https://www.sqlite.org/c3ref/load_extension.html).

Consider as an example, the [sample `upper`](_examples/upper/upper.go) module in `_examples/`. To build it, you'd use something similar to:

```shell
$ go build -buildmode=c-shared -o upper.so _examples/upper
```

which would emit an `upper.so` in the current directory. Now, to use it with (say) the `sqlite3` shell, you could do something like

```shell
$ sqlite3
> .load upper.so
> SELECT upper("sqlite3");
SQLITE3
> .exit
```

## Features

- [x] [`commit` / `rollback` hooks](https://www.sqlite.org/c3ref/commit_hook.html)
- [x] custom [`collation`](https://www.sqlite.org/c3ref/create_collation.html)
- [x] custom [`scalar`, `aggregate` and `window` functions](https://www.sqlite.org/appfunc.html)
- [x] custom [`virtual table`](https://www.sqlite.org/vtab.html) <sup>does not support `xFindFunction`, `xShadowName` and nested transations _yet_</sup>
- [ ] a simple [`sqlite3_exec`](https://www.sqlite.org/c3ref/exec.html) interface
- [ ] support [`sqlite3` session api](https://www.sqlite.org/sessionintro.html)
- [ ] support [`sqlite3` online backup api](https://www.sqlite.org/backup.html)

Each of the support feature provides an exported interface that the user code must implement. Refer to code and [godoc](https://pkg.go.dev/go.riyazali.net/sqlite)
for more details.

## License

MIT License Copyright (c) 2020 Riyaz Ali

Refer to [LICENSE](./LICENSE) for full text.

