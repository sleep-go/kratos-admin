package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

type fakeApplication struct {
	run func() error
}

func (a fakeApplication) Run() error { return a.run() }

func TestRunServerMigratesBeforeBuildingApplication(t *testing.T) {
	var calls []string
	dependencies := serverDependencies{
		load: func(string) (conf.Config, error) {
			calls = append(calls, "load")
			return conf.Config{Data: conf.Data{MySQLDSN: "dsn"}}, nil
		},
		migrate: func(context.Context, string) error {
			calls = append(calls, "migrate")
			return nil
		},
		newApplication: func(context.Context, conf.Config) (runnableApplication, func(), error) {
			calls = append(calls, "new")
			return fakeApplication{run: func() error {
				calls = append(calls, "run")
				return nil
			}}, func() { calls = append(calls, "cleanup") }, nil
		},
	}

	if err := runServerWith(context.Background(), "config.yaml", dependencies); err != nil {
		t.Fatal(err)
	}
	want := []string{"load", "migrate", "new", "run", "cleanup"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestRunServerStopsWhenMigrationFails(t *testing.T) {
	migrationErr := errors.New("migration failed")
	newApplicationCalled := false
	dependencies := serverDependencies{
		load:    func(string) (conf.Config, error) { return conf.Config{}, nil },
		migrate: func(context.Context, string) error { return migrationErr },
		newApplication: func(context.Context, conf.Config) (runnableApplication, func(), error) {
			newApplicationCalled = true
			return nil, nil, nil
		},
	}

	err := runServerWith(context.Background(), "config.yaml", dependencies)
	if !errors.Is(err, migrationErr) || newApplicationCalled {
		t.Fatalf("error = %v, newApplication called = %v", err, newApplicationCalled)
	}
}
