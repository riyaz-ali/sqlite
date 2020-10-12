#ifndef _BRIDGE_H
#define _BRIDGE_H

// This file defines a bridge between Golang and sqlite's c extension api.
// Most of sqlite api function defined in <sqlite3ext.h> are macros that redirect calls
// to an instance of sqlite3_api_routines, called as sqlite3_api.

// As neither macros nor c function pointers work directly in cgo, we need to define a bridge
// to redirect calls from golang to sqlite.

// Most of the methods follow the convention of prefixing the sqlite api function with an underscore.
// The bridge isn't extensive and doesn't cover the whole sqlite api.

#include "../sqlite3ext.h"

// aggregate routines
void* _sqlite3_aggregate_context(sqlite3_context *, int);

// query interface
int _sqlite3_exec(sqlite3 *, const char *, sqlite3_callback, void *, char **);

// prepared statement
int _sqlite3_prepare_v2(sqlite3 *, const char *, int, sqlite3_stmt **, const char **);
int _sqlite3_step(sqlite3_stmt *);
int _sqlite3_reset(sqlite3_stmt *);
int _sqlite3_clear_bindings(sqlite3_stmt *);

// binding values to prepared statement
int _sqlite3_bind_blob(sqlite3_stmt *, int, const void *, int, void (*)(void *));
int _sqlite3_bind_double(sqlite3_stmt *, int, double);
int _sqlite3_bind_int(sqlite3_stmt *, int, int);
int _sqlite3_bind_int64(sqlite3_stmt *, int, sqlite_int64);
int _sqlite3_bind_null(sqlite3_stmt *, int);
int _sqlite3_bind_text(sqlite3_stmt *, int, const char *, int, void (*)(void *));
int _sqlite3_bind_value(sqlite3_stmt *, int, const sqlite3_value *);
int _sqlite3_bind_zeroblob(sqlite3_stmt *, int, int);
int _sqlite3_bind_zeroblob64(sqlite3_stmt *, int, sqlite3_uint64);
int _sqlite3_bind_pointer(sqlite3_stmt *, int, void *, const char *, void (*)(void *));

int _sqlite3_bind_parameter_count(sqlite3_stmt *);
int _sqlite3_bind_parameter_index(sqlite3_stmt *, const char *);
const char* _sqlite3_bind_parameter_name(sqlite3_stmt *, int);

// getting result values from a query
int _sqlite3_data_count(sqlite3_stmt *);
const void *_sqlite3_value_blob(sqlite3_value *);
double _sqlite3_value_double(sqlite3_value *);
int _sqlite3_value_int(sqlite3_value *);
sqlite_int64 _sqlite3_value_int64(sqlite3_value *);
const unsigned char* _sqlite3_value_text(sqlite3_value *);
int _sqlite3_value_bytes(sqlite3_value *);
int _sqlite3_value_type(sqlite3_value *);
int _sqlite3_value_numeric_type(sqlite3_value *);
void* _sqlite3_value_pointer(sqlite3_value *, const char *);

// returning results from custom functions
void _sqlite3_result_blob(sqlite3_context *, const void *, int, void (*)(void *));
void _sqlite3_result_blob64(sqlite3_context *, const void *, sqlite3_uint64, void (*)(void *));
void _sqlite3_result_double(sqlite3_context *, double);
void _sqlite3_result_error(sqlite3_context *, const char *, int);
void _sqlite3_result_error_code(sqlite3_context *, int);
void _sqlite3_result_error_nomem(sqlite3_context *);
void _sqlite3_result_error_toobig(sqlite3_context *);
void _sqlite3_result_int(sqlite3_context *, int);
void _sqlite3_result_int64(sqlite3_context *, sqlite_int64);
void _sqlite3_result_null(sqlite3_context *);
void _sqlite3_result_text(sqlite3_context *, const char *, int, void (*)(void *));
void _sqlite3_result_value(sqlite3_context *, sqlite3_value *);
void _sqlite3_result_pointer(sqlite3_context *, void *, const char *, void (*)(void *));
void _sqlite3_result_zeroblob(sqlite3_context *, int);
int _sqlite3_result_zeroblob64(sqlite3_context *, sqlite3_uint64);

// source of data in a query result
const char *_sqlite3_column_database_name(sqlite3_stmt *, int);
const char *_sqlite3_column_table_name(sqlite3_stmt *, int);
const char *_sqlite3_column_origin_name(sqlite3_stmt *, int);

// create_* routines
int _sqlite3_create_collation_v2(sqlite3 *, const char *, int, void *, int (*)(void *, int, const void *, int, const void *), void (*)(void *));
int _sqlite3_create_function_v2(sqlite3 *, const char *, int, int, void *, void (*)(sqlite3_context *, int, sqlite3_value **), void (*)(sqlite3_context *, int, sqlite3_value **), void (*)(sqlite3_context *), void (*)(void *));
int _sqlite3_create_window_function(sqlite3 *, const char *, int, int, void *, void (*)(sqlite3_context *, int, sqlite3_value **), void (*)(sqlite3_context *), void (*)(sqlite3_context *), void (*)(sqlite3_context *, int, sqlite3_value **), void (*)(void *));
void *_sqlite3_user_data(sqlite3_context *);

// associate arbitrary metadata with a context
void* _sqlite3_get_auxdata(sqlite3_context *, int);
void  _sqlite3_set_auxdata(sqlite3_context *, int, void *, void (*)(void *));

// memory related operations
void  _sqlite3_free(void *);
void* _sqlite3_malloc(int);
void* _sqlite3_realloc(void *, int);

// error details handler
int _sqlite3_errcode(sqlite3 *);
const char *_sqlite3_errmsg(sqlite3 *);

// auth+tracing
int _sqlite3_set_authorizer(sqlite3 *, int (*)(void *, int, const char *, const char *, const char *, const char *), void *);
int _sqlite3_trace_v2(sqlite3 *, unsigned int, int (*)(unsigned int, void *, void *, void *), void *);

// hooks
void* _sqlite3_commit_hook(sqlite3 *, int (*)(void *), void *);
void* _sqlite3_rollback_hook(sqlite3 *, void (*)(void *), void *);
void* _sqlite3_update_hook(sqlite3 *, void (*)(void *, int, const char *, const char *, sqlite_int64), void *);

// status routines
int _sqlite3_status(int, int *, int *, int);
int _sqlite3_db_status(sqlite3 *, int, int *, int *, int);
int _sqlite3_stmt_status(sqlite3_stmt *, int, int);

// version number information
sqlite_int64 _sqlite3_last_insert_rowid(sqlite3 *);
const char* _sqlite3_libversion(void);
int _sqlite3_libversion_number(void);

// miscellaneous routines
int _sqlite3_get_autocommit(sqlite3 *);
int _sqlite3_enable_shared_cache(int);
void _sqlite3_interrupt(sqlite3 *);
int _sqlite3_release_memory(int);
int _sqlite3_threadsafe(void);


#if defined(BRIDGE_ENABLE_VTAB) // virtual table
int _sqlite3_create_module_v2(sqlite3 *, const char *, const sqlite3_module *, void *, void (*)(void *));
int _sqlite3_declare_vtab(sqlite3 *, const char *);
int _sqlite3_vtab_nochange(sqlite3_context *);
const char* _sqlite3_vtab_collation(sqlite3_index_info *, int);
int _sqlite3_overload_function(sqlite3 *, const char *, int);
int _sqlite3_vtab_config(sqlite3 *, int, ...);
int _sqlite3_vtab_on_conflict(sqlite3 *);
#endif

#if defined(BRIDGE_ENABLE_BLOB_IO) // Blob I/O
int _sqlite3_blob_open(sqlite3 *, const char *, const char *, const char *, sqlite3_int64, int, sqlite3_blob **);
int _sqlite3_blob_close(sqlite3_blob *);
int _sqlite3_blob_reopen(sqlite3_blob *, sqlite3_int64);
int _sqlite3_blob_bytes(sqlite3_blob *);
int _sqlite3_blob_read(sqlite3_blob *, void *, int, int);
int _sqlite3_blob_write(sqlite3_blob *, const void *, int, int);
#endif

#if defined(BRIDGE_ENABLE_VFS) // VFS
sqlite3_vfs *_sqlite3_vfs_find(const char *);
int _sqlite3_vfs_register(sqlite3_vfs *, int);
int _sqlite3_vfs_unregister(sqlite3_vfs *);
// database file information
const char *_sqlite3_filename_database(const char *);
const char *_sqlite3_filename_journal(const char *);
const char *_sqlite3_filename_wal(const char *);
#endif

#if defined(BRIDGE_ENABLE_BACKUP) // Backup
sqlite3_backup *_sqlite3_backup_init(sqlite3 *, const char *, sqlite3 *, const char *);
int _sqlite3_backup_finish(sqlite3_backup *);
int _sqlite3_backup_pagecount(sqlite3_backup *);
int _sqlite3_backup_remaining(sqlite3_backup *);
int _sqlite3_backup_step(sqlite3_backup *, int);
#endif

#endif // _BRIDGE_H