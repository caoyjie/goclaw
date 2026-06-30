package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/mediaremote"
)

func mediaRemoteConfigFromAppConfig(cfg *config.Config) mediaremote.Config {
	if cfg == nil {
		return mediaremote.Config{}
	}
	src := cfg.Media.ObjectStorage
	presignTTL := time.Duration(src.PresignTTLSeconds) * time.Second
	if presignTTL <= 0 {
		presignTTL = mediaremote.DefaultPresignTTL
	}
	maxDownloadBytes := src.MaxDownloadBytes
	if maxDownloadBytes <= 0 {
		maxDownloadBytes = mediaremote.DefaultMaxDownloadBytes
	}
	provider := src.Provider
	if provider == "" {
		provider = mediaremote.DefaultProvider
	}
	prefix := src.Prefix
	if prefix == "" {
		prefix = mediaremote.DefaultPrefix
	}
	urlMode := src.URLMode
	if urlMode == "" {
		urlMode = string(mediaremote.URLModePresigned)
	}
	return mediaremote.Config{
		Enabled:              src.Enabled,
		Provider:             provider,
		Bucket:               src.Bucket,
		Endpoint:             src.Endpoint,
		Region:               src.Region,
		Prefix:               prefix,
		AccessKeyID:          src.AccessKeyID,
		SecretAccessKey:      src.SecretAccessKey,
		URLMode:              mediaremote.URLMode(urlMode),
		PresignTTL:           presignTTL,
		RetentionDays:        src.RetentionDays,
		MaxDownloadBytes:     maxDownloadBytes,
		PublicBaseURL:        src.PublicBaseURL,
		AllowedDownloadHosts: append([]string(nil), src.AllowedDownloadHosts...),
	}
}

func buildRemoteMediaService(ctx context.Context, cfg *config.Config) (*mediaremote.Service, error) {
	remoteCfg := mediaRemoteConfigFromAppConfig(cfg)
	if !remoteCfg.Enabled {
		return nil, nil
	}
	if err := remoteCfg.Validate(); err != nil {
		return nil, err
	}
	storage, err := mediaremote.NewTOSClient(ctx, remoteCfg)
	if err != nil {
		return nil, fmt.Errorf("create media object storage client: %w", err)
	}
	return mediaremote.NewService(remoteCfg, mediaremote.HTTPDownloader{}, storage), nil
}
