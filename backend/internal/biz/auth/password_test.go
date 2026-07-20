package auth

import "testing"

func TestPasswordHasherHashAndVerify(t *testing.T) {
	hasher := NewPasswordHasher(DefaultPasswordParams())

	first, err := hasher.Hash("StrongPassword!2026")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	second, err := hasher.Hash("StrongPassword!2026")
	if err != nil {
		t.Fatalf("Hash() second error = %v", err)
	}
	if first == second {
		t.Fatal("Hash() must use a unique salt")
	}

	valid, err := hasher.Verify("StrongPassword!2026", first)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !valid {
		t.Fatal("Verify() = false, want true")
	}

	valid, err = hasher.Verify("WrongPassword", first)
	if err != nil {
		t.Fatalf("Verify() wrong password error = %v", err)
	}
	if valid {
		t.Fatal("Verify() wrong password = true, want false")
	}
}

func TestPasswordHasherRejectsMalformedHash(t *testing.T) {
	hasher := NewPasswordHasher(DefaultPasswordParams())

	if _, err := hasher.Verify("password", "not-an-argon2-hash"); err == nil {
		t.Fatal("Verify() error = nil, want malformed hash error")
	}
}
