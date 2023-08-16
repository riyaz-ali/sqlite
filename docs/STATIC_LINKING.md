# 🔗 Static Linking

> Static linking is the process of linking in parts of a static library when generating
> an object file or executable output <sup>[[source]](https://wiki.linuxquestions.org/wiki/Static_linking)</sup>

[`sqlite3` supports static linking](https://www.sqlite.org/loadext.html#statically_linking_a_run_time_loadable_extension)
your extension into the final application build. This way you do not have to fiddle around
with shipping multiple files (one for your application and another for your extension).

This approach is also a work-around if your host application is also written in Golang
(as outlined in [#16](https://github.com/riyaz-ali/sqlite/issues/16)), as you cannot load multiple Go runtimes in the same process.

There are two known ways in which you can statically link and initialize your extension.

### 1. Automatically Loading Extensions For All Connections

In this approach, you provide `sqlite3` amalgamation source, along with some other
boilerplate, and use [`sqlite3_auto_extension()`](https://www.sqlite.org/c3ref/auto_extension.html)
(via `cgo`) to automatically set up the extension for all new connections opened in the process.

One known project making use of this pattern is [`mergestat/mergestat-lite`](https://github.com/mergestat/mergestat-lite).

There, the bulk of the functionality is implemented as `sqlite3` virtual-table and bundled as
an extension (under [`/extensions`](/mergestat/mergestat-lite/tree/main/extensions) directory).
The extension is then statically linked into the final executable.

Specific point-of interests in that repository would be:

- [`/pkg/sqlite/sqlite.go`](https://github.com/mergestat/mergestat-lite/blob/main/pkg/sqlite/sqlite.go) where the extension is setup to [load automatically for each connection](https://www.sqlite.org/c3ref/auto_extension.html). This package also contains the `sqlite3` amalgamation source.
- [`/cmd/setup.go`](https://github.com/mergestat/mergestat-lite/blob/main/cmd/setup.go#L10-L14) where the package is linked to the main application (using side-effect import).
- [`/Makefile`](https://github.com/mergestat/mergestat-lite/blob/main/Makefile#L6-L12) that contains the relevant linker flags to allow compiling the intermediate object files with unresolved symbols (this is to workaround the way `go build` works for `cgo`)

### 2. Manually Registering With Each Connection

In this approach, you need a supported `sqlite3` driver that provides access to the underlying `sqlite3` database pointer.

You can apply the following patch to [`crawshaw.io/sqlite`](https://github.com/crawshaw/sqlite) to make it compatible.

```patch
---
 sqlite.go | 3 +++
 1 file changed, 3 insertions(+)

diff --git a/sqlite.go b/sqlite.go
index 8283a68..f441fa0 100644
--- a/sqlite.go
+++ b/sqlite.go
@@ -188,6 +188,9 @@ func (conn *Conn) Close() error {
 	return reserr("Conn.Close", "", "", res)
 }
 
+// UnderlyingConnection returns the underlying C.sqlite3 database connection object.
+func (conn *Conn) UnderlyingConnection() unsafe.Pointer { return unsafe.Pointer(conn.conn) }
+
 func (conn *Conn) GetAutocommit() bool {
 	return int(C.sqlite3_get_autocommit(conn.conn)) != 0
 }
```

Then, you can use [`RegisterWith()`](https://pkg.go.dev/go.riyazali.net/sqlite#RegisterWith) to register the extension
manually with each connection.

```golang
conn, _ := sqlite.OpenConn("file:test.db?mode=memory") // crawshaw.io's connection
defer conn.Close()

uc := ext.UnderlyingConnection(conn.UnderlyingConnection())  // ext.UnderlyingConnection() is a type-cast
err := ext.RegisterWith(uc, func(api *ext.ExtensionApi) (ext.ErrorCode, error) {
	if err := api.CreateFunction("go_upper", &Upper{}); err != nil {
		return ext.SQLITE_ERROR, err
	}
	return ext.SQLITE_OK, nil
})
```

To build now, run:

```shell
> go build -tags=static .  # build with static tag
```

See [#18](https://github.com/riyaz-ali/sqlite/issues/18) more details.