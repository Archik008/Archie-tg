package tgclient

import "testing"

func TestNormalizePhoneRussia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "+79918856513", want: "+79918856513"},
		{in: "79918856513", want: "+79918856513"},
		{in: "89918856513", want: "+79918856513"},
		{in: "+89918856513", want: "+79918856513"},
		{in: "  +7 991 885 65 13 ", want: "+79918856513"},
		{in: "+1234567890", want: "+1234567890"},
	}

	for _, tt := range tests {
		if got := normalizePhone(tt.in); got != tt.want {
			t.Fatalf("normalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidatePhoneRussia(t *testing.T) {
	t.Parallel()

	if err := validatePhone("+79918856513"); err != nil {
		t.Fatalf("expected valid RU phone, got %v", err)
	}
	if err := validatePhone("89918856513"); err != nil {
		t.Fatalf("expected valid RU phone from 8-prefix, got %v", err)
	}
	if err := validatePhone("+89918856513"); err != nil {
		t.Fatalf("expected +899... to normalize to valid RU phone, got %v", err)
	}
}
