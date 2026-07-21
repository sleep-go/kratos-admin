package data

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

type fakeLockRow struct {
	value int64
	err   error
}

func (r fakeLockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int64)) = r.value
	return nil
}

type fakeLockConnection struct {
	lockResult int64
	scanErr    error
	releaseErr error
	calls      []string
}

func (c *fakeLockConnection) QueryRowContext(context.Context, string, ...any) lockRow {
	c.calls = append(c.calls, "GET_LOCK")
	return fakeLockRow{value: c.lockResult, err: c.scanErr}
}

func (c *fakeLockConnection) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	c.calls = append(c.calls, "RELEASE_LOCK")
	return nil, c.releaseErr
}

func TestRunLockedMigrationRunsAndReleasesInOrder(t *testing.T) {
	connection := &fakeLockConnection{lockResult: 1}
	err := runLockedMigration(context.Background(), connection, func(context.Context) error {
		connection.calls = append(connection.calls, "UP")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"GET_LOCK", "UP", "RELEASE_LOCK"}
	if !reflect.DeepEqual(connection.calls, want) {
		t.Fatalf("calls = %v, want %v", connection.calls, want)
	}
}

func TestRunLockedMigrationRejectsLockTimeout(t *testing.T) {
	connection := &fakeLockConnection{lockResult: 0}
	called := false
	err := runLockedMigration(context.Background(), connection, func(context.Context) error {
		called = true
		return nil
	})
	if err == nil || called {
		t.Fatalf("error = %v, up called = %v", err, called)
	}
	if len(connection.calls) != 1 || connection.calls[0] != "GET_LOCK" {
		t.Fatalf("calls = %v", connection.calls)
	}
}

func TestRunLockedMigrationReturnsOperationErrors(t *testing.T) {
	tests := []struct {
		name       string
		connection *fakeLockConnection
		upErr      error
	}{
		{name: "scan", connection: &fakeLockConnection{scanErr: errors.New("scan")}},
		{name: "up", connection: &fakeLockConnection{lockResult: 1}, upErr: errors.New("up")},
		{name: "release", connection: &fakeLockConnection{lockResult: 1, releaseErr: errors.New("release")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runLockedMigration(context.Background(), test.connection, func(context.Context) error {
				return test.upErr
			})
			if err == nil {
				t.Fatal("error = nil")
			}
		})
	}
}
