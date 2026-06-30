package mediaremote

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64, mimeType string) error
	URL(ctx context.Context, key string, ttl time.Duration) (string, *time.Time, error)
}

type TOSClient struct {
	client    *s3.Client
	presigner *s3.PresignClient
	cfg       Config
}

func NewTOSClient(ctx context.Context, cfg Config) (*TOSClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	var opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}
	client := s3.NewFromConfig(awsCfg, opts...)
	return &TOSClient{client: client, presigner: s3.NewPresignClient(client), cfg: cfg}, nil
}

func (c *TOSClient) Upload(ctx context.Context, key string, body io.Reader, size int64, mimeType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.cfg.Bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(mimeType),
	})
	if err != nil {
		return fmt.Errorf("tos upload %q: %w", key, err)
	}
	return nil
}

func (c *TOSClient) URL(ctx context.Context, key string, ttl time.Duration) (string, *time.Time, error) {
	mode := c.cfg.URLMode
	if mode == "" {
		mode = URLModePresigned
	}
	if mode == URLModePublic {
		return publicObjectURL(c.cfg, key)
	}
	if ttl <= 0 {
		ttl = DefaultPresignTTL
	}
	resp, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", nil, fmt.Errorf("tos presign %q: %w", key, err)
	}
	expiresAt := time.Now().UTC().Add(ttl)
	return resp.URL, &expiresAt, nil
}

type publicURLStorage struct {
	cfg Config
}

func NewPublicURLStorage(cfg Config) ObjectStorage {
	return publicURLStorage{cfg: cfg}
}

func (s publicURLStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, mimeType string) error {
	return nil
}

func (s publicURLStorage) URL(ctx context.Context, key string, ttl time.Duration) (string, *time.Time, error) {
	return publicObjectURL(s.cfg, key)
}

func publicObjectURL(cfg Config, key string) (string, *time.Time, error) {
	if cfg.URLMode != "" && cfg.URLMode != URLModePublic {
		return "", nil, fmt.Errorf("public URL requested for %s mode", cfg.URLMode)
	}
	base := cfg.PublicBaseURL
	if base == "" {
		if cfg.Endpoint == "" || cfg.Bucket == "" {
			return "", nil, fmt.Errorf("public URL requires public_base_url or endpoint and bucket")
		}
		base = strings.TrimRight(cfg.Endpoint, "/") + "/" + pathEscape(cfg.Bucket)
	}
	return strings.TrimRight(base, "/") + "/" + escapeKeyPath(key), nil, nil
}

func escapeKeyPath(key string) string {
	parts := strings.Split(strings.TrimLeft(key, "/"), "/")
	for i, part := range parts {
		parts[i] = pathEscape(part)
	}
	return strings.Join(parts, "/")
}

func pathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}
