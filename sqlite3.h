// this is an indirection to import proper header file
// actual header used is in h/

#ifndef _SQLITE3_H
#define _SQLITE3_H

#ifdef USE_LIBSQLITE3
#include <sqlite3.h>
#include <sqlite3ext.h>
#else
#include "h/sqlite3.h"
#include "h/sqlite3ext.h"
#endif

#endif