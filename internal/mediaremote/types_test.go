package mediaremote

import (
	"strings"
	"testing"
	"time"
)

func TestObjectKeyBuildsTenantScopedPath(t *testing.T) {
	key, err := ObjectKey("goclaw-media/", KeyContext{
		TenantID:       "tenant-1",
		AgentID:        "ceo-assistant",
		SessionKeyHash: "a91f2c7e",
		Now:            time.Date(2026, 6, 29, 10, 11, 12, 0, time.UTC),
		MediaID:        "0192f6e8-7f9e-7a80-bc83-2ff4c6bb7d0a",
	}, "image/png")
	if err != nil {
		t.Fatalf("ObjectKey returned error: %v", err)
	}

	want := "goclaw-media/tenants/tenant-1/agents/ceo-assistant/sessions/a91f2c7e/2026/06/29/0192f6e8-7f9e-7a80-bc83-2ff4c6bb7d0a.png"
	if key != want {
		t.Fatalf("ObjectKey = %q, want %q", key, want)
	}
}

func TestObjectKeyRejectsUnsafeSegments(t *testing.T) {
	_, err := ObjectKey("goclaw-media/", KeyContext{
		TenantID:       "tenant/1",
		AgentID:        "ceo-assistant",
		SessionKeyHash: "a91f2c7e",
		Now:            time.Now(),
		MediaID:        "media-1",
	}, "image/png")
	if err == nil {
		t.Fatal("ObjectKey returned nil error for unsafe tenant id")
	}
	if !strings.Contains(err.Error(), "tenant") {
		t.Fatalf("error = %q, want tenant context", err.Error())
	}
}

func TestObjectKeyRejectsUnsupportedMIME(t *testing.T) {
	_, err := ObjectKey("goclaw-media/", KeyContext{
		TenantID:       "tenant-1",
		AgentID:        "ceo-assistant",
		SessionKeyHash: "a91f2c7e",
		Now:            time.Now(),
		MediaID:        "media-1",
	}, "application/pdf")
	if err == nil {
		t.Fatal("ObjectKey returned nil error for unsupported MIME")
	}
}

func TestConfigValidateDefaultsURLMode(t *testing.T) {
	cfg := Config{
		Enabled:         true,
		Provider:        "tos",
		Bucket:          "goclaw-media",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
