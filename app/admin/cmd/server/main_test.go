package main

import (
	"context"
	"errors"
	"testing"
)

func TestRootCommandReturnsRunnerError(t *testing.T) {
	want := errors.New("启动失败")
	cmd := newRootCommand(func(context.Context) error { return want })
	if err := cmd.ExecuteContext(context.Background()); !errors.Is(err, want) {
		t.Fatalf("ExecuteContext() error = %v, want %v", err, want)
	}
	if cmd.Use != "admin-server" {
		t.Fatalf("Use = %q, want admin-server", cmd.Use)
	}
}
