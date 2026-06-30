package mediaremote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var ErrCrossHostRedirect = errors.New("cross-host redirect rejected")

type DownloadedMedia struct {
	Path     string
	MimeType string
	Size     int64
	NameHint string
}

type Downloader interface {
	Download(ctx context.Context, candidate Candidate, cfg Config) (DownloadedMedia, error)
}

type HTTPDownloader struct {
	Client *http.Client
}

func (d HTTPDownloader) Download(ctx context.Context, candidate Candidate, cfg Config) (DownloadedMedia, error) {
	return DownloadCandidateWithClient(ctx, candidate, cfg, d.Client)
}

func DownloadCandidate(ctx context.Context, candidate Candidate, cfg Config) (DownloadedMedia, error) {
	return DownloadCandidateWithClient(ctx, candidate, cfg, nil)
}

func DownloadCandidateWithClient(ctx context.Context, candidate Candidate, cfg Config, client *http.Client) (DownloadedMedia, error) {
	u, err := url.Parse(candidate.SourceURL)
	if err != nil {
		return DownloadedMedia{}, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return DownloadedMedia{}, fmt.Errorf("unsupported download URL scheme %q", u.Scheme)
	}
	if err := checkAllowedHost(u.Hostname(), cfg.AllowedDownloadHosts); err != nil {
		return DownloadedMedia{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	} else {
		copyClient := *client
		client = &copyClient
		if client.Timeout == 0 {
			client.Timeout = 30 * time.Second
		}
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		if req.URL.Host != via[0].URL.Host {
			return ErrCrossHostRedirect
		}
		return checkAllowedHost(req.URL.Hostname(), cfg.AllowedDownloadHosts)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.SourceURL, nil)
	if err != nil {
		return DownloadedMedia{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return DownloadedMedia{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DownloadedMedia{}, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	maxBytes := cfg.MaxDownloadBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxDownloadBytes
	}
	tmp, err := os.CreateTemp("", "goclaw-remote-media-*")
	if err != nil {
		return DownloadedMedia{}, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	limited := io.LimitReader(resp.Body, maxBytes+1)
	var sniff bytes.Buffer
	size, err := copyWithSniff(tmp, limited, &sniff)
	if err != nil {
		return DownloadedMedia{}, err
	}
	if size > maxBytes {
		return DownloadedMedia{}, fmt.Errorf("download exceeds max size %d bytes", maxBytes)
	}
	if err := tmp.Close(); err != nil {
		return DownloadedMedia{}, err
	}

	mimeType := normalizeMediaMIME(candidate.MimeType)
	if mimeType == "" {
		mimeType = normalizeMediaMIME(resp.Header.Get("Content-Type"))
	}
	if mimeType == "" {
		mimeType = normalizeMediaMIME(http.DetectContentType(sniff.Bytes()))
	}
	if mimeType == "" {
		return DownloadedMedia{}, fmt.Errorf("unsupported downloaded media MIME type")
	}

	cleanup = false
	return DownloadedMedia{
		Path:     tmpPath,
		MimeType: mimeType,
		Size:     size,
		NameHint: candidate.NameHint,
	}, nil
}

func copyWithSniff(dst io.Writer, src io.Reader, sniff *bytes.Buffer) (int64, error) {
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if sniff.Len() < 512 {
				remaining := 512 - sniff.Len()
				if remaining > len(chunk) {
					remaining = len(chunk)
				}
				_, _ = sniff.Write(chunk[:remaining])
			}
			written, writeErr := dst.Write(chunk)
			size += int64(written)
			if writeErr != nil {
				return size, writeErr
			}
			if written != n {
				return size, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return size, nil
			}
			return size, readErr
		}
	}
}

func checkAllowedHost(host string, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	for _, allowedHost := range allowed {
		if strings.EqualFold(host, allowedHost) {
			return nil
		}
	}
	return fmt.Errorf("download host %q is not allowed", host)
}

func normalizeMediaMIME(value string) string {
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp", "video/mp4":
		return mimeType
	default:
		return ""
	}
}
