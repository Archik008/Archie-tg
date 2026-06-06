package tgclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "+1234567890", want: "+1234567890"},
		{in: "1234567890", want: "+1234567890"},
		{in: "  +79990001122 ", want: "+79990001122"},
		{in: "   ", want: ""},
	}

	for _, tt := range tests {
		if got := normalizePhone(tt.in); got != tt.want {
			t.Fatalf("normalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestClearSessionFilesRemovesSavedSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	client, err := NewClient(dir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	sessionPath := filepath.Join(dir, "tg.session")
	metaPath := filepath.Join(dir, "session.meta.json")

	if err := os.WriteFile(sessionPath, []byte("session"), 0o600); err != nil {
		t.Fatalf("WriteFile session: %v", err)
	}
	if err := client.SaveSessionMeta(SessionMeta{
		AppID:   1,
		AppHash: "hash",
		Phone:   "+10000000000",
	}); err != nil {
		t.Fatalf("SaveSessionMeta: %v", err)
	}

	if err := client.removeSessionFiles(sessionPath); err != nil {
		t.Fatalf("removeSessionFiles: %v", err)
	}
	if err := client.removeSessionFiles(metaPath); err != nil {
		t.Fatalf("removeSessionFiles meta: %v", err)
	}

	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("session file still exists: %v", err)
	}
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Fatalf("meta file still exists: %v", err)
	}
}
