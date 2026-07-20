package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

type memoryCaptchaStore struct {
	values map[string]string
}

func (s *memoryCaptchaStore) Put(_ context.Context, id, answerHash string, _ time.Duration) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[id] = answerHash
	return nil
}

func (s *memoryCaptchaStore) Take(_ context.Context, id string) (string, error) {
	value := s.values[id]
	delete(s.values, id)
	return value, nil
}

func TestCaptchaGenerateAndOneTimeVerify(t *testing.T) {
	store := &memoryCaptchaStore{}
	usecase := NewCaptchaUsecase(store, func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) })
	challenge, err := usecase.Generate(context.Background())
	if err != nil || challenge.ID == "" || !strings.HasPrefix(challenge.ImageURI, "data:image/png;base64,") {
		t.Fatalf("Generate() = %+v, %v", challenge, err)
	}
	store.values[challenge.ID] = captchaHash(challenge.ID, "12345")
	if err := usecase.Verify(context.Background(), challenge.ID, "12345"); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := usecase.Verify(context.Background(), challenge.ID, "12345"); err != ErrCaptchaInvalid {
		t.Fatalf("repeated Verify() error = %v", err)
	}
}

func TestCaptchaWrongAnswerConsumesChallenge(t *testing.T) {
	store := &memoryCaptchaStore{values: map[string]string{"id": captchaHash("id", "12345")}}
	usecase := NewCaptchaUsecase(store, nil)
	if err := usecase.Verify(context.Background(), "id", "54321"); err != ErrCaptchaInvalid {
		t.Fatalf("Verify() error = %v", err)
	}
	if _, ok := store.values["id"]; ok {
		t.Fatal("wrong answer must consume challenge")
	}
}
