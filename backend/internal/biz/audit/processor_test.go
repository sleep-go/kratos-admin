package audit

import (
	"context"
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
