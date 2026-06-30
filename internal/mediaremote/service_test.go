package mediaremote

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

type fakeDownloader struct {
	path string
	err  error
}

func (f fakeDownloader) Download(ctx context.Context, candidate Candidate, cfg Config) (DownloadedMedia, error) {
	if f.err != nil {
		return DownloadedMedia{}, f.err
	}
	return DownloadedMedia{Path: f.path, MimeType: "image/png", Size: 12}, nil
}

type fakeStorage struct {
	uploadedKey string
	err         error
}

func (f *fakeStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, mimeType string) error {
	f.uploadedKey = key
	return f.err
}

func (f *fakeStorage) URL(ctx context.Context, key string, ttl time.Duration) (string, *time.Time, error) {
	return "https://media.example.com/" + key, nil, nil
}

func TestServiceDisabledReturnsNoRefs(t *testing.T) {
	svc := NewService(Config{}, nil, nil)
	got, err := svc.ProcessToolResult(context.Background(), ToolResultInput{
		ToolName: "comfy_get_output",
		Raw:      `{"url":"https://example.com/a.png"}`,
	})
	if err != nil {
		t.Fatalf("ProcessToolResult returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("refs = %d, want 0", len(got))
	}
}

func TestServiceUploadsComfyImage(t *testing.T) {
	tmp := writeTempFile(t, []byte("fake png"))
	storage := &fakeStorage{}
	svc := NewService(Config{
		Enabled:         true,
		Provider:        "tos",
		Bucket:          "bucket",
		Prefix:          "goclaw-media/",
		URLMode:         URLModePublic,
		PresignTTL:      time.Hour,
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	}, fakeDownloader{path: tmp}, storage)

	got, err := svc.ProcessToolResult(context.Background(), ToolResultInput{
		ToolName:       "comfy_get_output",
		Raw:            `{"url":"https://example.com/a.png","mime_type":"image/png"}`,
		TenantID:       "tenant-1",
		AgentID:        "ceo-assistant",
		SessionKeyHash: "a91f2c7e",
		Now:            time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC),
		MediaID:        "media-1",
	})
	if err != nil {
		t.Fatalf("ProcessToolResult returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("refs = %d, want 1", len(got))
	}
	if got[0].URL == "" || got[0].Key == "" {
		t.Fatalf("ref missing URL or Key: %+v", got[0])
	}
	if storage.uploadedKey != got[0].Key {
		t.Fatalf("uploadedKey = %q, ref key = %q", storage.uploadedKey, got[0].Key)
	}
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestServiceReturnsNoRefsForNonComfyTool(t *testing.T) {
	storage := &fakeStorage{}
	svc := NewService(Config{Enabled: true}, fakeDownloader{}, storage)
	got, err := svc.ProcessToolResult(context.Background(), ToolResultInput{
		ToolName: "read_file",
		Raw:      `{"url":"https://example.com/a.png"}`,
	})
	if err != nil {
		t.Fatalf("ProcessToolResult returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("refs = %d, want 0", len(got))
	}
}

func TestServiceUploadFailureRemovesTempFile(t *testing.T) {
	tmp := writeTempFile(t, []byte("fake png"))
	storage := &fakeStorage{err: errors.New("upload failed")}
	svc := NewService(Config{Enabled: true, Prefix: "goclaw-media/", URLMode: URLModePublic}, fakeDownloader{path: tmp}, storage)

	_, err := svc.ProcessToolResult(context.Background(), ToolResultInput{
		ToolName:       "comfy_get_output",
		Raw:            `{"url":"https://example.com/a.png","mime_type":"image/png"}`,
		TenantID:       "tenant-1",
		AgentID:        "ceo-assistant",
		SessionKeyHash: "a91f2c7e",
		Now:            time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC),
		MediaID:        "media-1",
	})
	if err == nil {
		t.Fatal("ProcessToolResult returned nil error")
	}
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file still exists or stat failed unexpectedly: %v", err)
	}
}

func writeTempFile(t *testing.T, body []byte) string {
	t.Helper()
	tmp, err := os.CreateTemp("", "goclaw-mediaremote-test-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	return tmp.Name()
}
