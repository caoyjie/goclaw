package mediaremote

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTOSPublicURLUsesPublicBaseURL(t *testing.T) {
	client := NewPublicURLStorage(Config{
		Bucket:        "bucket",
		PublicBaseURL: "https://cdn.example.com/media/",
		URLMode:       URLModePublic,
	})

	got, expiresAt, err := client.URL(context.Background(), "goclaw-media/a.png", time.Hour)
	if err != nil {
		t.Fatalf("URL returned error: %v", err)
	}
	if expiresAt != nil {
		t.Fatalf("expiresAt = %v, want nil", expiresAt)
	}
	if got != "https://cdn.example.com/media/goclaw-media/a.png" {
		t.Fatalf("URL = %q", got)
	}
}

func TestTOSPublicURLFallsBackToEndpointPathStyle(t *testing.T) {
	client := NewPublicURLStorage(Config{
		Bucket:   "bucket",
		Endpoint: "https://tos-s3-cn-beijing.volces.com",
		URLMode:  URLModePublic,
	})

	got, _, err := client.URL(context.Background(), "goclaw-media/a.png", time.Hour)
	if err != nil {
		t.Fatalf("URL returned error: %v", err)
	}
	if got != "https://tos-s3-cn-beijing.volces.com/bucket/goclaw-media/a.png" {
		t.Fatalf("URL = %q", got)
	}
}

func TestTOSPublicURLRejectsPresignedMode(t *testing.T) {
	client := NewPublicURLStorage(Config{URLMode: URLModePresigned})
	_, _, err := client.URL(context.Background(), "a.png", time.Hour)
	if err == nil {
		t.Fatal("URL returned nil error")
	}
	if !strings.Contains(err.Error(), "presigned") {
		t.Fatalf("error = %q, want presigned context", err.Error())
	}
}
