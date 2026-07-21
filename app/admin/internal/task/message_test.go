package task

import (
	"encoding/json"
	"testing"
)

func TestTaskMessageConstructors(t *testing.T) {
	tests := []struct {
		name      string
		build     func() (Message, error)
		wantID    string
		wantType  string
		wantField string
		wantValue any
	}{
		{name: "audit", build: func() (Message, error) { return NewAuditMessage("event-1") }, wantID: "audit:event-1", wantType: RoutingAudit, wantField: "event_id", wantValue: "event-1"},
		{name: "log export", build: func() (Message, error) { return NewLogExportMessage("export-1") }, wantID: "log-export:export-1", wantType: RoutingLogExport, wantField: "export_id", wantValue: "export-1"},
		{name: "file cleanup", build: func() (Message, error) {
			return NewFileCleanupMessage(7, "file-1", "local", "7/file-1.txt")
		}, wantID: "file-cleanup:file-1", wantType: RoutingFileCleanup, wantField: "file_id", wantValue: "file-1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, err := test.build()
			if err != nil {
				t.Fatal(err)
			}
			if message.ID != test.wantID || message.Type != test.wantType {
				t.Fatalf("message = %+v", message)
			}
			var payload map[string]any
			if err := json.Unmarshal(message.Body, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["version"] != float64(1) || payload[test.wantField] != test.wantValue {
				t.Fatalf("payload = %#v", payload)
			}
		})
	}
}

func TestTaskMessageConstructorsRejectInvalidInput(t *testing.T) {
	tests := []func() (Message, error){
		func() (Message, error) { return NewAuditMessage("") },
		func() (Message, error) { return NewLogExportMessage("") },
		func() (Message, error) { return NewFileCleanupMessage(0, "", "", "") },
	}
	for index, build := range tests {
		if _, err := build(); err == nil {
			t.Fatalf("case %d error = nil", index)
		}
	}
}
