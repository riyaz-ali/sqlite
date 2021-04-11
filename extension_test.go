package sqlite_test

import (
	"errors"
	"fmt"
	. "go.riyazali.net/sqlite"
	"testing"
)

func TestRegister(t *testing.T) {
	Register(func(api *ExtensionApi) (ErrorCode, error) {
		return SQLITE_ERROR, errors.New("test: controlled failure")
	})

	if _, err := Connect(Memory); err == nil {
		t.FailNow() // it should report back the errors
	}
}

func TestVersion(t *testing.T) {
	Register(func(api *ExtensionApi) (ErrorCode, error) {
		if version := api.Version(); version < 3034000 {
			return SQLITE_ERROR, fmt.Errorf("reported version is %d", version)
		}
		return SQLITE_OK, nil
	})

	if db, err := Connect(Memory); err != nil {
		t.Fatal(err)
	} else {
		_ = db.Close()
	}
}

func TestAutoCommit(t *testing.T) {
	Register(func(api *ExtensionApi) (ErrorCode, error) {
		var c = api.Connection()

		if !api.AutoCommit() { // autocommit is true outside of transaction
			return SQLITE_ERROR, errors.New("autocommit() must report true")
		}

		if err := c.Exec("BEGIN", nil); err != nil {
			return SQLITE_ERROR, err
		}

		if api.AutoCommit() { // autocommit is false within a transaction
			return SQLITE_ERROR, errors.New("autocommit() must report false")
		}

		if err := c.Exec("ROLLBACK", nil); err != nil {
			return SQLITE_ERROR, err
		}

		return SQLITE_OK, nil
	})

	if db, err := Connect(Memory); err != nil {
		t.Fatal(err)
	} else {
		_ = db.Close()
	}
}

func TestLimit(t *testing.T) {
	Register(func(api *ExtensionApi) (ErrorCode, error) {
		var value = api.Limit(LIMIT_ATTACHED)
		if value != 10 { // 10 is the default
			return SQLITE_ERROR, errors.New("mismatched limit value")
		}

		_ = api.SetLimit(LIMIT_ATTACHED, 5)
		value = api.Limit(LIMIT_ATTACHED)
		if value != 5 { // 5 is the new value
			return SQLITE_ERROR, errors.New("must have updated limit value")
		}

		return SQLITE_OK, nil
	})

	if db, err := Connect(Memory); err != nil {
		t.Fatal(err)
	} else {
		_ = db.Close()
	}
}

