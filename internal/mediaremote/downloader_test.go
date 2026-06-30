package mediaremote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDownloadCandidateWritesTempFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nsmall"))
	}))
	t.Cleanup(srv.Close)

	got, err := DownloadCandidate(context.Background(), Candidate{SourceURL: srv.URL + "/out.png"}, Config{MaxDownloadBytes: 1024})
	if err != nil {
		t.Fatalf("DownloadCandidate returned error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(got.Path) })

	if got.Path == "" {
		t.Fatal("Path is empty")
	}
	if got.MimeType != "image/png" {
		t.Fatalf("MimeType = %q, want image/png", got.MimeType)
	}
	if got.Size == 0 {
		t.Fatal("Size is zero")
	}
}

func TestDownloadCandidateRejectsMaxBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("0123456789"))
	}))
	t.Cleanup(srv.Close)

	_, err := DownloadCandidate(context.Background(), Candidate{SourceURL: srv.URL}, Config{MaxDownloadBytes: 4})
	if err == nil {
		t.Fatal("DownloadCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %q, want exceeds", err.Error())
	}
}

func TestDownloadCandidateRejectsCrossHostRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	_, err := DownloadCandidate(context.Background(), Candidate{SourceURL: redirector.URL}, Config{MaxDownloadBytes: 1024})
	if err == nil {
		t.Fatal("DownloadCandidate returned nil error")
	}
	if !errors.Is(err, ErrCrossHostRedirect) {
		t.Fatalf("error = %v, want ErrCrossHostRedirect", err)
	}
}

func TestDownloadCandidateRejectsNonHTTP(t *testing.T) {
	_, err := DownloadCandidate(context.Background(), Candidate{SourceURL: "file:///tmp/out.png"}, Config{})
	if err == nil {
		t.Fatal("DownloadCandidate returned nil error")
	}
}
