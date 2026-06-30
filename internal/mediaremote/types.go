package mediaremote

import (
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"
)

type URLMode string

const (
	URLModePublic    URLMode = "public"
	URLModePresigned URLMode = "presigned"

	DefaultProvider         = "tos"
	DefaultPrefix           = "goclaw-media/"
	DefaultPresignTTL       = 24 * time.Hour
	DefaultMaxDownloadBytes = 50 * 1024 * 1024
)

type Config struct {
	Enabled              bool
	Provider             string
	Bucket               string
	Endpoint             string
	Region               string
	Prefix               string
	AccessKeyID          string
	SecretAccessKey      string
	URLMode              URLMode
	PresignTTL           time.Duration
	RetentionDays        int
	MaxDownloadBytes     int64
	PublicBaseURL        string
	AllowedDownloadHosts []string
}

type Candidate struct {
	SourceURL string
	MimeType  string
	NameHint  string
	Prompt    string
	ToolName  string
}

type RemoteMediaRef struct {
	URL       string     `json:"url"`
	Key       string     `json:"key"`
	MimeType  string     `json:"mime_type"`
	Kind      string     `json:"kind"`
	Size      int64      `json:"size"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Prompt    string     `json:"prompt,omitempty"`
}

type KeyContext struct {
	TenantID       string
	AgentID        string
	SessionKeyHash string
	Now            time.Time
	MediaID        string
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Provider != "" && c.Provider != DefaultProvider {
		return fmt.Errorf("unsupported media object storage provider %q", c.Provider)
	}
	if c.Bucket == "" {
		return fmt.Errorf("media object storage bucket is required")
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return fmt.Errorf("media object storage credentials are required")
	}
	mode := c.URLMode
	if mode == "" {
		mode = URLModePresigned
	}
	if mode != URLModePublic && mode != URLModePresigned {
		return fmt.Errorf("unsupported media object storage url_mode %q", c.URLMode)
	}
	return nil
}

func ObjectKey(prefix string, kc KeyContext, mimeType string) (string, error) {
	ext, err := extensionForMIME(mimeType)
	if err != nil {
		return "", err
	}
	if err := validatePathSegment("tenant_id", kc.TenantID); err != nil {
		return "", err
	}
	if err := validatePathSegment("agent_id", kc.AgentID); err != nil {
		return "", err
	}
	if err := validatePathSegment("session_key_hash", kc.SessionKeyHash); err != nil {
		return "", err
	}
	if err := validatePathSegment("media_id", kc.MediaID); err != nil {
		return "", err
	}
	now := kc.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	prefix = strings.TrimLeft(prefix, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	key := path.Join(
		prefix,
		"tenants", kc.TenantID,
		"agents", kc.AgentID,
		"sessions", kc.SessionKeyHash,
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", int(now.Month())),
		fmt.Sprintf("%02d", now.Day()),
		kc.MediaID+ext,
	)
	if strings.HasSuffix(prefix, "/") && !strings.HasPrefix(key, prefix) {
		key = prefix + strings.TrimPrefix(key, strings.TrimSuffix(prefix, "/")+"/")
	}
	return key, nil
}

func extensionForMIME(mimeType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png", nil
	case "image/jpeg":
		return ".jpg", nil
	case "image/webp":
		return ".webp", nil
	case "video/mp4":
		return ".mp4", nil
	default:
		return "", fmt.Errorf("unsupported media MIME type %q", mimeType)
	}
}

func kindForMIME(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	default:
		return ""
	}
}

func validatePathSegment(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.Contains(value, "/") || strings.Contains(value, `\`) || strings.Contains(value, "..") {
		return fmt.Errorf("%s contains unsafe path characters", name)
	}
	for _, r := range value {
		if unicode.IsSpace(r) {
			return fmt.Errorf("%s contains whitespace", name)
		}
	}
	return nil
}
