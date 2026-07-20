package main

import (
	"context"
	"errors"
	"testing"
)

func TestRootCommandReturnsRunnerError(t *testing.T) {
	want := errors.New("启动失败")
	var gotConf string
	cmd := newRootCommand(func(_ context.Context, confPath string) error {
		gotConf = confPath
		return want
	})
	if err := cmd.ExecuteContext(context.Background()); !errors.Is(err, want) {
		t.Fatalf("ExecuteContext() error = %v, want %v", err, want)
	}
	if cmd.Use != "admin-server" {
		t.Fatalf("Use = %q, want admin-server", cmd.Use)
	}
	if gotConf != "./configs/admin.yaml" {
		t.Fatalf("conf = %q, want ./configs/admin.yaml", gotConf)
	}
}

func TestRootCommandParsesConfFlag(t *testing.T) {
	var gotConf string
	cmd := newRootCommand(func(_ context.Context, confPath string) error {
		gotConf = confPath
		return nil
	})
	cmd.SetArgs([]string{"--conf", "/etc/kratos-admin/admin.yaml"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotConf != "/etc/kratos-admin/admin.yaml" {
		t.Fatalf("conf = %q", gotConf)
	}
}
