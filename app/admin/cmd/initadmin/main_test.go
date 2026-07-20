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
		"--conf", "/etc/kratos-admin/admin.yaml",
		"--username", "root",
		"--display-name", "平台管理员",
		"--email", "root@example.com",
		"--phone", "13800000000",
	})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.Conf != "/etc/kratos-admin/admin.yaml" || got.Username != "root" || got.DisplayName != "平台管理员" || got.Email != "root@example.com" || got.Phone != "13800000000" {
		t.Fatalf("options = %+v", got)
	}
}

func TestRootCommandUsesDefaultConf(t *testing.T) {
	var got initAdminOptions
	cmd := newRootCommand(func(_ context.Context, options initAdminOptions) error {
		got = options
		return nil
	})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.Conf != "./configs/admin.yaml" {
		t.Fatalf("conf = %q, want ./configs/admin.yaml", got.Conf)
	}
}
