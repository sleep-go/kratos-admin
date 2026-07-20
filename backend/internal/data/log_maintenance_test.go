package data

import "testing"

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
