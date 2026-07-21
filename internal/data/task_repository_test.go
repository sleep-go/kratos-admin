package data

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type permanentFailure string

func (e permanentFailure) Error() string { return string(e) }
func (permanentFailure) Permanent() bool { return true }

func TestTaskRepositoryMergePendingTasks(t *testing.T) {
	base := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	tasks := mergePendingTasks(3,
		[]PendingTask{{Kind: TaskKindAudit, ID: "audit-2", CreatedAt: base.Add(2 * time.Minute)}},
		[]PendingTask{
			{Kind: TaskKindLogExport, ID: "export-1", CreatedAt: base},
			{Kind: TaskKindLogExport, ID: "export-4", CreatedAt: base.Add(4 * time.Minute)},
		},
		[]PendingTask{{Kind: TaskKindFileCleanup, ID: "file-1", CreatedAt: base.Add(time.Minute)}},
	)

	if len(tasks) != 3 {
		t.Fatalf("待办任务数量 = %d，期望 3", len(tasks))
	}
	want := []string{"export-1", "file-1", "audit-2"}
	for index, id := range want {
		if tasks[index].ID != id {
			t.Fatalf("第 %d 个任务 = %s，期望 %s", index, tasks[index].ID, id)
		}
	}
}

func TestTaskRepositoryFailureState(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	state := nextTaskFailure(0, errors.New("临时失败"), now)
	if state.RetryCount != 1 || state.Final {
		t.Fatalf("首次失败状态 = %+v", state)
	}
	if state.NextRetryAt == nil || !state.NextRetryAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("首次重试时间 = %v", state.NextRetryAt)
	}

	state = nextTaskFailure(9, errors.New(strings.Repeat("错", 1100)), now)
	if state.RetryCount != 10 || !state.Final || state.NextRetryAt != nil {
		t.Fatalf("第十次失败状态 = %+v", state)
	}
	if len([]rune(state.Reason)) != taskFailureReasonLimit {
		t.Fatalf("错误长度 = %d，期望 %d", len([]rune(state.Reason)), taskFailureReasonLimit)
	}
}

func TestTaskRepositoryPermanentFailureStopsRetrying(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	state := nextTaskFailure(0, permanentFailure("配置永久不匹配"), now)
	if !state.Final || state.RetryCount != 1 || state.NextRetryAt != nil {
		t.Fatalf("永久失败状态 = %+v", state)
	}
}

func TestTaskRepositoryIdempotencyKey(t *testing.T) {
	tests := []struct {
		task PendingTask
		want string
	}{
		{task: PendingTask{Kind: TaskKindAudit, ID: "a1"}, want: "audit:a1"},
		{task: PendingTask{Kind: TaskKindLogExport, ID: "l1"}, want: "log-export:l1"},
		{task: PendingTask{Kind: TaskKindFileCleanup, ID: "f1"}, want: "file-cleanup:f1"},
	}
	for _, test := range tests {
		if got := taskIdempotencyKey(test.task); got != test.want {
			t.Fatalf("幂等键 = %s，期望 %s", got, test.want)
		}
	}
}
