package data

import (
	"context"
	"reflect"
	"testing"
)

func TestRetentionDaysAcceptsSafeRange(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want uint32
		ok   bool
	}{
		{raw: `30`, want: 30, ok: true},
		{raw: `"365"`, want: 365, ok: true},
		{raw: `0`, ok: false},
		{raw: `3651`, ok: false},
		{raw: `"invalid"`, ok: false},
	} {
		got, ok := retentionDays([]byte(test.raw))
		if got != test.want || ok != test.ok {
			t.Fatalf("retentionDays(%s) = %d, %v", test.raw, got, ok)
		}
	}
}

func TestRunLockedMaintenanceSkipsWhenAnotherInstanceOwnsLock(t *testing.T) {
	connection := &fakeLockConnection{lockResult: 0}
	called := false
	result, err := runLockedMaintenance(context.Background(), connection, func(context.Context) (LogCleanupResult, error) {
		called = true
		return LogCleanupResult{}, nil
	})
	if err != nil || called || result != (LogCleanupResult{}) {
		t.Fatalf("result=%+v called=%v error=%v", result, called, err)
	}
	if !reflect.DeepEqual(connection.calls, []string{"GET_LOCK"}) {
		t.Fatalf("calls=%v", connection.calls)
	}
}

func TestRunLockedMaintenanceCleansAndReleases(t *testing.T) {
	connection := &fakeLockConnection{lockResult: 1}
	want := LogCleanupResult{Audit: 1, Login: 2, API: 3}
	result, err := runLockedMaintenance(context.Background(), connection, func(context.Context) (LogCleanupResult, error) {
		connection.calls = append(connection.calls, "CLEANUP")
		return want, nil
	})
	if err != nil || result != want {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if !reflect.DeepEqual(connection.calls, []string{"GET_LOCK", "CLEANUP", "RELEASE_LOCK"}) {
		t.Fatalf("calls=%v", connection.calls)
	}
}
