package audit

import (
	"context"
	"fmt"
	"testing"
)

type fakeRepository struct {
	event     Event
	published bool
	writes    int
}

func (r *fakeRepository) Get(_ context.Context, _ string) (Event, error) { return r.event, nil }
func (r *fakeRepository) Publish(_ context.Context, _ Event, _ Entry) (bool, error) {
	if r.published {
		return false, nil
	}
	r.published = true
	r.writes++
	return true, nil
}

func TestProcessorPersistsImpersonatorID(t *testing.T) {
	var captured Entry
	repository := &capturingRepository{
		event: Event{ID: "event", TenantID: 8, Payload: []byte(`{"user_id":5,"member_id":9,"impersonator_id":1,"action":"update","resource_type":"roles","resource_id":"3"}`)},
		onPublish: func(_ Event, entry Entry) { captured = entry },
	}
	processor := NewProcessor(repository)
	if err := processor.Process(context.Background(), "event"); err != nil {
		t.Fatal(err)
	}
	if captured.ImpersonatorID != 1 {
		t.Fatalf("ImpersonatorID = %d, want 1", captured.ImpersonatorID)
	}
}

type capturingRepository struct {
	event     Event
	published bool
	onPublish func(Event, Entry)
}

func (r *capturingRepository) Get(_ context.Context, _ string) (Event, error) { return r.event, nil }
func (r *capturingRepository) Publish(_ context.Context, event Event, entry Entry) (bool, error) {
	if r.published {
		return false, nil
	}
	r.published = true
	if r.onPublish != nil {
		r.onPublish(event, entry)
	}
	return true, nil
}

func TestProcessorGeneratesAuditIdempotently(t *testing.T) {
	repository := &fakeRepository{event: Event{ID: "event", TenantID: 8, Payload: []byte(`{"user_id":1,"member_id":2,"action":"update","resource_type":"roles","resource_id":"3","summary":"更新角色"}`)}}
	processor := NewProcessor(repository)

	if err := processor.Process(context.Background(), "event"); err != nil {
		t.Fatal(err)
	}
	if err := processor.Process(context.Background(), "event"); err != nil {
		t.Fatal(err)
	}
	if repository.writes != 1 {
		t.Fatalf("audit writes = %d, want 1", repository.writes)
	}
}

func TestProcessorValidatesRealmConsistency(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  uint64
		realm     string
		wantError bool
	}{
		{name: "platform with tenant", tenantID: 5, realm: "platform", wantError: true},
		{name: "tenant without tenant", tenantID: 0, realm: "tenant", wantError: true},
		{name: "platform without tenant", tenantID: 0, realm: "platform", wantError: false},
		{name: "tenant with tenant", tenantID: 8, realm: "tenant", wantError: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"user_id":1,"realm":%q,"action":"update","resource_type":"roles","resource_id":"3"}`, tt.realm)
			repository := &capturingRepository{
				event: Event{ID: "event", TenantID: tt.tenantID, Payload: []byte(payload)},
			}
			processor := NewProcessor(repository)
			err := processor.Process(context.Background(), "event")
			if tt.wantError && err == nil {
				t.Fatalf("Process() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("Process() unexpected error: %v", err)
			}
			if !tt.wantError && !repository.published {
				t.Fatalf("Process() expected publish, but not published")
			}
		})
	}
}
