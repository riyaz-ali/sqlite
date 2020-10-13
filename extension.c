// This file contains the SQLite3 extension entry-point routine
// as defined here https://sqlite.org/loadext.html
#include "sqlite3.h"

SQLITE_EXTENSION_INIT1

// hook to call into golang functionality
// defined in extension.go
extern int go_sqlite3_extension_init(sqlite3*, char**);

#ifdef _WIN32
__declspec(dllexport)
#endif
#ifdef SQLITE3_INIT_FN
  int SQLITE3_INIT_FN
#else
  int sqlite3_extension_init
#endif
(sqlite3* db, char** pzErrMsg, const sqlite3_api_routines *pApi) {
	SQLITE_EXTENSION_INIT2(pApi)
	return go_sqlite3_extension_init(db, pzErrMsg);
}