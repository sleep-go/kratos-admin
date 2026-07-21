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
