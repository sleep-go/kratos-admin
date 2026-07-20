package main

import (
	"context"
	"testing"
)

func TestRootCommandParsesOutputPath(t *testing.T) {
	var got genOptions
	cmd := newRootCommand(func(_ context.Context, options genOptions) error {
		got = options
		return nil
	})
	cmd.SetArgs([]string{"--out-path", "internal/data/query"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.OutPath != "internal/data/query" {
		t.Fatalf("OutPath = %q", got.OutPath)
	}
}
