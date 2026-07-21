package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeMessageSender struct {
	channel string
	last    CodeMessage
}

func (s *fakeMessageSender) Channel() string { return s.channel }
func (s *fakeMessageSender) SendCode(_ context.Context, message CodeMessage) error {
	s.last = message
	return nil
}

type fakeVerificationRepository struct {
	record       VerificationRecord
	wantHash     string
	passwordHash string
	userID       uint64
}

func TestMFAChallengeNeverStoresCredentials(t *testing.T) {
	repository := &fakeVerificationRepository{}
	users := &fakeUserRepository{user: &User{ID: 8, Email: "admin@example.com", Status: UserStatusEnabled}}
	sender := &fakeMessageSender{channel: "email"}
	usecase, err := NewVerificationUsecase(repository, users, NewPasswordHasher(DefaultPasswordParams()), []byte("0123456789abcdef"), []MessageSender{sender}, nil)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := usecase.IssueMFA(context.Background(), User{ID: 8, Email: "admin@example.com", MFAChannel: "email"}, LoginInput{
		Identifier: "admin", Password: "NeverPersistThis!", DeviceName: "Safari", IP: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(repository.record.ContextData), "NeverPersistThis") || strings.Contains(string(repository.record.ContextData), "admin") {
		t.Fatalf("MFA context leaks credentials: %s", repository.record.ContextData)
	}
	repository.wantHash = usecase.codeHash(sender.last.Code)
	user, input, err := usecase.VerifyMFA(context.Background(), challenge.ID, sender.last.Code)
	if err != nil || user.ID != 8 || input.DeviceName != "Safari" || input.Password != "" {
		t.Fatalf("VerifyMFA() = user %+v input %+v err %v", user, input, err)
	}
}

func (r *fakeVerificationRepository) CreateVerification(_ context.Context, record VerificationRecord, _ time.Duration) (uint64, error) {
	record.ID = 9
	r.record = record
	return 9, nil
}
func (r *fakeVerificationRepository) ConsumeVerification(_ context.Context, id uint64, scene, codeHash string, _ time.Time, _ uint32) (VerificationRecord, error) {
	if id != r.record.ID || scene != r.record.Scene || codeHash != r.wantHash {
		return VerificationRecord{}, ErrVerificationInvalid
	}
	return r.record, nil
}
func (r *fakeVerificationRepository) UpdatePassword(_ context.Context, userID uint64, passwordHash string, _ time.Time) error {
	r.userID, r.passwordHash = userID, passwordHash
	return nil
}

func TestPasswordResetUsesOneTimeCodeAndRevokesSessions(t *testing.T) {
	repository := &fakeVerificationRepository{}
	users := &fakeUserRepository{user: &User{ID: 8, Email: "admin@example.com"}}
	sender := &fakeMessageSender{channel: "email"}
	hasher := NewPasswordHasher(PasswordParams{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16})
	usecase, err := NewVerificationUsecase(repository, users, hasher, []byte("0123456789abcdef"), []MessageSender{sender}, nil)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := usecase.ForgotPassword(context.Background(), "admin", "email")
	if err != nil || challenge.ID != 9 {
		t.Fatalf("ForgotPassword() = %+v, %v", challenge, err)
	}
	if sender.last.Code == "" {
		t.Fatal("verification code was not delivered")
	}
	repository.wantHash = usecase.codeHash(sender.last.Code)
	if err := usecase.ResetPassword(context.Background(), challenge.ID, sender.last.Code, "NewPassword!2026"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if repository.userID != 8 || repository.passwordHash == "" {
		t.Fatalf("password update = user %d hash %q", repository.userID, repository.passwordHash)
	}
}
