package data

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeLockRow struct {
	value int64
	err   error
}

func TestApplyMigrationsUsesProvidedDatabase(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationLockSerializesConcurrentMySQLMigrations(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 并发迁移集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			errorsChannel <- Migrate(ctx, dsn)
		}()
	}
	ready.Wait()
	close(start)
	for range 2 {
		if err := <-errorsChannel; err != nil {
			t.Fatalf("并发迁移失败: %v", err)
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var applied int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT version_id) FROM goose_db_version WHERE is_applied = 1 AND version_id BETWEEN 1 AND 6").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 6 {
		t.Fatalf("已应用迁移数量 = %d，期望 6", applied)
	}
}

func (r fakeLockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int64)) = r.value
	return nil
}

type fakeLockConnection struct {
	lockResult    int64
	scanErr       error
	releaseResult *int64
	releaseErr    error
	calls         []string
}

func (c *fakeLockConnection) QueryRowContext(_ context.Context, query string, _ ...any) lockRow {
	if strings.Contains(query, "RELEASE_LOCK") {
		c.calls = append(c.calls, "RELEASE_LOCK")
		result := int64(1)
		if c.releaseResult != nil {
			result = *c.releaseResult
		}
		return fakeLockRow{value: result, err: c.releaseErr}
	}
	c.calls = append(c.calls, "GET_LOCK")
	return fakeLockRow{value: c.lockResult, err: c.scanErr}
}

func TestRunLockedMigrationRejectsUnconfirmedRelease(t *testing.T) {
	notReleased := int64(0)
	connection := &fakeLockConnection{lockResult: 1, releaseResult: &notReleased}
	if err := runLockedMigration(context.Background(), connection, func(context.Context) error { return nil }); err == nil {
		t.Fatal("RELEASE_LOCK 返回 0 时 error = nil")
	}
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
