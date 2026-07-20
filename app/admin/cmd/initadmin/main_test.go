package main

import (
	"context"
	"testing"
)

func TestRootCommandParsesAdminOptions(t *testing.T) {
	var got initAdminOptions
	cmd := newRootCommand(func(_ context.Context, options initAdminOptions) error {
		got = options
		return nil
	})
	cmd.SetArgs([]string{
		"--username", "root",
		"--display-name", "平台管理员",
		"--email", "root@example.com",
		"--phone", "13800000000",
	})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.Username != "root" || got.DisplayName != "平台管理员" || got.Email != "root@example.com" || got.Phone != "13800000000" {
		t.Fatalf("options = %+v", got)
	}
}
