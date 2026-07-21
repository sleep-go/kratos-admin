package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

var temporaryDatabaseNamePattern = regexp.MustCompile(`^kratos_admin_gen_[a-z0-9_]+$`)

type temporaryDatabaseAdmin interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Close() error
}

type temporaryDatabaseOpener func(string) (temporaryDatabaseAdmin, error)

func temporaryDatabaseName(now time.Time, entropy []byte) string {
	return fmt.Sprintf("kratos_admin_gen_%d_%s", now.UnixNano(), hex.EncodeToString(entropy))
}

func temporaryDatabaseDSNs(sourceDSN string) (serverDSN, databaseDSN, name string, err error) {
	config, err := mysqlDriver.ParseDSN(sourceDSN)
	if err != nil {
		return "", "", "", fmt.Errorf("解析代码生成数据库配置失败: %w", err)
	}
	entropy := make([]byte, 8)
	if _, err := rand.Read(entropy); err != nil {
		return "", "", "", fmt.Errorf("生成临时数据库名称失败: %w", err)
	}
	name = temporaryDatabaseName(time.Now().UTC(), entropy)
	if !temporaryDatabaseNamePattern.MatchString(name) {
		return "", "", "", fmt.Errorf("临时数据库名称不安全: %q", name)
	}
	serverConfig := config.Clone()
	serverConfig.DBName = ""
	databaseConfig := config.Clone()
	databaseConfig.DBName = name
	return serverConfig.FormatDSN(), databaseConfig.FormatDSN(), name, nil
}

func openSQLTemporaryDatabaseAdmin(dsn string) (temporaryDatabaseAdmin, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func withTemporaryDatabase(ctx context.Context, sourceDSN string, run func(string) error) error {
	return withTemporaryDatabaseUsing(ctx, sourceDSN, openSQLTemporaryDatabaseAdmin, run)
}

func withTemporaryDatabaseUsing(ctx context.Context, sourceDSN string, open temporaryDatabaseOpener, run func(string) error) (resultErr error) {
	serverDSN, databaseDSN, name, err := temporaryDatabaseDSNs(sourceDSN)
	if err != nil {
		return err
	}
	admin, err := open(serverDSN)
	if err != nil {
		return fmt.Errorf("打开代码生成数据库连接失败: %w", err)
	}
	defer func() {
		if closeErr := admin.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("关闭代码生成数据库连接失败: %w", closeErr))
		}
	}()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return fmt.Errorf("创建代码生成临时数据库失败: %w", err)
	}
	runErr := run(databaseDSN)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	_, cleanupErr := admin.ExecContext(cleanupCtx, "DROP DATABASE `"+name+"`")
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("删除代码生成临时数据库 %s 失败: %w", name, cleanupErr)
	}
	return errors.Join(runErr, cleanupErr)
}
