package setup

import (
	"context"
	"errors"
	"testing"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
)

type fakeAdminRepository struct {
	existing *Admin
	created  *Admin
}

func (r *fakeAdminRepository) FindByUsername(_ context.Context, _ string) (*Admin, error) {
	if r.existing == nil {
		return nil, ErrAdminNotFound
	}
	return r.existing, nil
}

func (r *fakeAdminRepository) Create(_ context.Context, admin Admin) error {
	r.created = &admin
	return nil
}

func TestInitializerCreatesPlatformAdminWithoutFixedMigrationPassword(t *testing.T) {
	repo := &fakeAdminRepository{}
	hasher := bizauth.NewPasswordHasher(bizauth.PasswordParams{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16})
	initializer := NewAdminInitializer(repo, hasher)

	created, err := initializer.Ensure(context.Background(), AdminInput{
		Username: "root", DisplayName: "平台管理员", Password: "StrongPassword!2026",
	})
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !created || repo.created == nil || !repo.created.PlatformAdmin {
		t.Fatalf("created = %v, admin = %+v", created, repo.created)
	}
	valid, err := hasher.Verify("StrongPassword!2026", repo.created.PasswordHash)
	if err != nil || !valid {
		t.Fatalf("created password hash invalid: valid=%v err=%v", valid, err)
	}
}

func TestInitializerIsIdempotentForExistingPlatformAdmin(t *testing.T) {
	repo := &fakeAdminRepository{existing: &Admin{Username: "root", PlatformAdmin: true}}
	initializer := NewAdminInitializer(repo, bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()))

	created, err := initializer.Ensure(context.Background(), AdminInput{Username: "root", Password: "StrongPassword!2026"})
	if err != nil || created {
		t.Fatalf("Ensure() = created %v, err %v", created, err)
	}
	if repo.created != nil {
		t.Fatal("existing platform admin must not be recreated")
	}
}

func TestInitializerRejectsExistingOrdinaryUser(t *testing.T) {
	repo := &fakeAdminRepository{existing: &Admin{Username: "root"}}
	initializer := NewAdminInitializer(repo, bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()))

	_, err := initializer.Ensure(context.Background(), AdminInput{Username: "root", Password: "StrongPassword!2026"})
	if !errors.Is(err, ErrUsernameOccupied) {
		t.Fatalf("Ensure() error = %v, want ErrUsernameOccupied", err)
	}
}
