package main

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type fakeTemporaryDatabaseAdmin struct {
	queries []string
	dropErr error
	closed  bool
}

func (a *fakeTemporaryDatabaseAdmin) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	a.queries = append(a.queries, query)
	if strings.HasPrefix(query, "DROP DATABASE") {
		return nil, a.dropErr
	}
	return nil, nil
}

func (a *fakeTemporaryDatabaseAdmin) Close() error {
	a.closed = true
	return nil
}

func TestTemporaryDatabaseNameIsSafe(t *testing.T) {
	name := temporaryDatabaseName(time.Unix(1721520000, 0), []byte{0x01, 0xab, 0xff})
	if !regexp.MustCompile(`^kratos_admin_gen_[a-z0-9_]+$`).MatchString(name) {
		t.Fatalf("临时数据库名不安全: %q", name)
	}
}

func TestTemporaryDatabaseAlwaysDropsAndJoinsErrors(t *testing.T) {
	runErr := errors.New("生成失败")
	dropErr := errors.New("清理失败")
	admin := &fakeTemporaryDatabaseAdmin{dropErr: dropErr}
	err := withTemporaryDatabaseUsing(
		context.Background(),
		"root:secret@tcp(127.0.0.1:3306)/business?parseTime=true",
		func(string) (temporaryDatabaseAdmin, error) { return admin, nil },
		func(string) error { return runErr },
	)
	if !errors.Is(err, runErr) || !errors.Is(err, dropErr) {
		t.Fatalf("error = %v，期望同时包含生成和清理错误", err)
	}
	if len(admin.queries) != 2 || !strings.HasPrefix(admin.queries[0], "CREATE DATABASE") || !strings.HasPrefix(admin.queries[1], "DROP DATABASE") {
		t.Fatalf("queries = %v", admin.queries)
	}
	if !admin.closed {
		t.Fatal("管理员连接未关闭")
	}
}

func TestTemporaryDatabaseDoesNotReuseSourceDatabaseName(t *testing.T) {
	admin := &fakeTemporaryDatabaseAdmin{}
	var generatedDSN string
	err := withTemporaryDatabaseUsing(
		context.Background(),
		"root:secret@tcp(127.0.0.1:3306)/business?parseTime=true",
		func(serverDSN string) (temporaryDatabaseAdmin, error) {
			config, parseErr := mysqlDriver.ParseDSN(serverDSN)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if config.DBName != "" {
				t.Fatalf("管理员 DSN 数据库名 = %q", config.DBName)
			}
			return admin, nil
		},
		func(dsn string) error {
			generatedDSN = dsn
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	config, err := mysqlDriver.ParseDSN(generatedDSN)
	if err != nil {
		t.Fatal(err)
	}
	if config.DBName == "" || config.DBName == "business" {
		t.Fatalf("生成 DSN 数据库名 = %q", config.DBName)
	}
}
