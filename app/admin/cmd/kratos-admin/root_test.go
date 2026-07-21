package main

import (
	"context"
	"testing"
)

func TestRootCommandProvidesAllSubcommands(t *testing.T) {
	cmd := newRootCommand(commandRunners{})
	want := map[string]bool{
		"server":     false,
		"init-admin": false,
		"gorm-gen":   false,
	}
	for _, child := range cmd.Commands() {
		if _, ok := want[child.Name()]; !ok {
			t.Fatalf("存在未预期子命令 %q", child.Name())
		}
		want[child.Name()] = true
	}
	for name, found := range want {
		if !found {
			t.Fatalf("缺少子命令 %q", name)
		}
	}
}

func TestServerCommandUsesDefaultConf(t *testing.T) {
	var got string
	cmd := newRootCommand(commandRunners{
		server: func(_ context.Context, confPath string) error {
			got = confPath
			return nil
		},
	})
	cmd.SetArgs([]string{"server"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "./configs/admin.yaml" {
		t.Fatalf("conf = %q, want ./configs/admin.yaml", got)
	}
}

func TestInitAdminCommandParsesOptions(t *testing.T) {
	var got initAdminOptions
	cmd := newRootCommand(commandRunners{
		initAdmin: func(_ context.Context, options initAdminOptions) error {
			got = options
			return nil
		},
	})
	cmd.SetArgs([]string{
		"init-admin",
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

func TestGORMGenCommandParsesOutputPath(t *testing.T) {
	var got genOptions
	cmd := newRootCommand(commandRunners{
		gormGen: func(_ context.Context, options genOptions) error {
			got = options
			return nil
		},
	})
	cmd.SetArgs([]string{"gorm-gen", "--out-path", "./tmp/query"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.OutPath != "./tmp/query" {
		t.Fatalf("OutPath = %q", got.OutPath)
	}
}

func TestSubcommandsRejectPositionalArguments(t *testing.T) {
	cmd := newRootCommand(commandRunners{
		server: func(context.Context, string) error { return nil },
	})
	cmd.SetArgs([]string{"server", "unexpected"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("ExecuteContext() error = nil, want positional argument error")
	}
}
