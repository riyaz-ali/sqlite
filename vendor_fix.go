// +build dummy

// This file is part of a workaround for `go mod vendor` which won't vendor
// C files if there's no Go file in the same directory.
// This would prevent the bridge/bridge.c file to be vendored.
//
// See this issue for reference: https://github.com/golang/go/issues/26366
package sqlite

import _ "go.riyazali.net/sqlite/h"
import _ "go.riyazali.net/sqlite/bridge"
