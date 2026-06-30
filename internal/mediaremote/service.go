package mediaremote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type ToolResultInput struct {
	ToolName       string
	Args           map[string]any
	Raw            string
	TenantID       string
	AgentID        string
	SessionKeyHash string
	Now            time.Time
	MediaID        string
}

type Service struct {
	cfg        Config
	downloader Downloader
	storage    ObjectStorage
}

func NewService(cfg Config, downloader Downloader, storage ObjectStorage) *Service {
	if downloader == nil {
		downloader = HTTPDownloader{}
	}
	return &Service{cfg: cfg, downloader: downloader, storage: storage}
}

func (s *Service) ProcessToolResult(ctx context.Context, input ToolResultInput) ([]RemoteMediaRef, error) {
	if s == nil || !s.cfg.Enabled {
		return nil, nil
	}
	candidates := ExtractCandidates(input.ToolName, input.Args, input.Raw)
	if len(candidates) == 0 {
		return nil, nil
	}
	if s.storage == nil {
		return nil, errors.New("media object storage is not configured")
	}

	var refs []RemoteMediaRef
	var failures []error
	for i, candidate := range candidates {
		ref, err := s.processCandidate(ctx, input, candidate, i)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 && len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	if len(failures) > 0 {
		return refs, errors.Join(failures...)
	}
	return refs, nil
}

func (s *Service) processCandidate(ctx context.Context, input ToolResultInput, candidate Candidate, index int) (RemoteMediaRef, error) {
	downloaded, err := s.downloader.Download(ctx, candidate, s.cfg)
	if err != nil {
		return RemoteMediaRef{}, err
	}
	defer os.Remove(downloaded.Path)

	mediaID := input.MediaID
	if mediaID == "" {
		mediaID = store.GenNewID().String()
		if index > 0 {
			mediaID = fmt.Sprintf("%s-%d", mediaID, index+1)
		}
	}
	key, err := ObjectKey(s.prefix(), KeyContext{
		TenantID:       input.TenantID,
		AgentID:        input.AgentID,
		SessionKeyHash: input.SessionKeyHash,
		Now:            input.Now,
		MediaID:        mediaID,
	}, downloaded.MimeType)
	if err != nil {
		return RemoteMediaRef{}, err
	}

	file, err := os.Open(downloaded.Path)
	if err != nil {
		return RemoteMediaRef{}, err
	}
	defer file.Close()

	if err := s.storage.Upload(ctx, key, file, downloaded.Size, downloaded.MimeType); err != nil {
		return RemoteMediaRef{}, err
	}
	url, expiresAt, err := s.storage.URL(ctx, key, s.cfg.PresignTTL)
	if err != nil {
		return RemoteMediaRef{}, err
	}
	return RemoteMediaRef{
		URL:       url,
		Key:       key,
		MimeType:  downloaded.MimeType,
		Kind:      kindForMIME(downloaded.MimeType),
		Size:      downloaded.Size,
		ExpiresAt: expiresAt,
		Prompt:    candidate.Prompt,
	}, nil
}

func (s *Service) prefix() string {
	prefix := s.cfg.Prefix
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return strings.TrimLeft(prefix, "/")
}
