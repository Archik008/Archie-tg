package tgclient

import "testing"

func TestValidateAppCredentials(t *testing.T) {
	t.Parallel()

	validHash := "0123456789abcdef0123456789abcdef"

	if err := validateAppCredentials(123, validHash); err != nil {
		t.Fatalf("expected valid credentials, got %v", err)
	}

	if err := validateAppCredentials(0, validHash); err == nil {
		t.Fatal("expected app_id validation error")
	}
	if err := validateAppCredentials(123, "short"); err == nil {
		t.Fatal("expected app_hash length validation error")
	}
	if err := validateAppCredentials(123, validHash+"!"); err == nil {
		t.Fatal("expected app_hash hex validation error")
	}
}

func TestMaskAppHash(t *testing.T) {
	t.Parallel()

	got := maskAppHash("0123456789abcdef0123456789abcdef")
	want := "01234567..."
	if got != want {
		t.Fatalf("maskAppHash() = %q, want %q", got, want)
	}
}
