// This file declares the wait_for_unlock_notify function.
// See the documentation on Stmt.Step.

#include "sqlite3.h"
#include <pthread.h>

typedef struct {
	int fired;
	pthread_cond_t cond;
	pthread_mutex_t mu;
} _unlock_note;

_unlock_note* _unlock_note_alloc();
void _unlock_note_fire(_unlock_note* un);
void _unlock_note_free(_unlock_note* un);

int _wait_for_unlock_notify(sqlite3 *db, _unlock_note* un);