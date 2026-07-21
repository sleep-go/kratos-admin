package main

import (
	"context"
	"testing"
)

func TestRootCommandProvidesAllSubcommands(t *testing.T) {
	cmd := newRootCommand(commandRunners{})
	want := map[string]bool{
		"migrate":    false,
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

func TestMigrateCommandUsesDefaultConf(t *testing.T) {
	var got string
	cmd := newRootCommand(commandRunners{
		migrate: func(_ context.Context, confPath string) error {
			got = confPath
			return nil
		},
	})
	cmd.SetArgs([]string{"migrate"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "./configs/config.yaml" {
		t.Fatalf("conf = %q, want ./configs/config.yaml", got)
	}
}

func TestInitAdminCommandUsesCustomConf(t *testing.T) {
	var got string
	cmd := newRootCommand(commandRunners{
		initAdmin: func(_ context.Context, confPath string) error {
			got = confPath
			return nil
		},
	})
	cmd.SetArgs([]string{"init-admin", "--conf", "/etc/kratos-admin/config.yaml"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "/etc/kratos-admin/config.yaml" {
		t.Fatalf("conf = %q", got)
	}
}

func TestGORMGenCommandParsesOptions(t *testing.T) {
	var got genOptions
	cmd := newRootCommand(commandRunners{
		gormGen: func(_ context.Context, options genOptions) error {
			got = options
			return nil
		},
	})
	cmd.SetArgs([]string{"gorm-gen", "--conf", "./tmp/config.yaml", "--model-out-path", "./tmp/model", "--query-out-path", "./tmp/query"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.ConfPath != "./tmp/config.yaml" || got.ModelOutPath != "./tmp/model" || got.QueryOutPath != "./tmp/query" {
		t.Fatalf("options = %+v", got)
	}
}

func TestSubcommandsRejectPositionalArguments(t *testing.T) {
	cmd := newRootCommand(commandRunners{
		migrate: func(context.Context, string) error { return nil },
	})
	cmd.SetArgs([]string{"migrate", "unexpected"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatal("ExecuteContext() error = nil, want positional argument error")
	}
}
