package config

import "testing"

func TestMediaObjectStorageDBSecrets(t *testing.T) {
	cfg := Default()
	cfg.ApplyDBSecrets(map[string]string{
		"media.object_storage.access_key_id":     "media-ak",
		"media.object_storage.secret_access_key": "media-sk",
	})

	if cfg.Media.ObjectStorage.AccessKeyID != "media-ak" {
		t.Fatalf("access key = %q", cfg.Media.ObjectStorage.AccessKeyID)
	}
	if cfg.Media.ObjectStorage.SecretAccessKey != "media-sk" {
		t.Fatalf("secret key = %q", cfg.Media.ObjectStorage.SecretAccessKey)
	}
}

func TestExtractDBSecretsIncludesOnlyMediaObjectStorageSecrets(t *testing.T) {
	cfg := Default()
	cfg.Media.ObjectStorage.AccessKeyID = "media-ak"
	cfg.Media.ObjectStorage.SecretAccessKey = "media-sk"
	cfg.Media.ObjectStorage.Bucket = "not-secret"

	secrets := cfg.ExtractDBSecrets()
	if secrets["media.object_storage.access_key_id"] != "media-ak" {
		t.Fatalf("missing media access key secret: %#v", secrets)
	}
	if secrets["media.object_storage.secret_access_key"] != "media-sk" {
		t.Fatalf("missing media secret access key: %#v", secrets)
	}
	if _, ok := secrets["backup.s3.access_key_id"]; ok {
		t.Fatalf("media secret extraction should not produce backup key: %#v", secrets)
	}
	if _, ok := secrets["media.object_storage.bucket"]; ok {
		t.Fatalf("bucket should not be extracted as a secret: %#v", secrets)
	}
}

func TestApplySystemConfigsMediaObjectStorage(t *testing.T) {
	cfg := Default()
	cfg.ApplySystemConfigs(map[string]string{
		"media.object_storage.enabled":                "true",
		"media.object_storage.provider":               "tos",
		"media.object_storage.bucket":                 "goclaw-media",
		"media.object_storage.endpoint":               "https://tos.example.com",
		"media.object_storage.region":                 "cn-beijing",
		"media.object_storage.prefix":                 "media/",
		"media.object_storage.url_mode":               "public",
		"media.object_storage.public_base_url":        "https://cdn.example.com",
		"media.object_storage.presign_ttl_seconds":    "604800",
		"media.object_storage.retention_days":         "7",
		"media.object_storage.max_download_bytes":     "1048576",
		"media.object_storage.allowed_download_hosts": `["storage.googleapis.com","cloud.comfy.org"]`,
	})

	got := cfg.Media.ObjectStorage
	if !got.Enabled || got.Bucket != "goclaw-media" || got.URLMode != "public" {
		t.Fatalf("system configs not applied: %+v", got)
	}
	if got.PresignTTLSeconds != 604800 || got.RetentionDays != 7 || got.MaxDownloadBytes != 1048576 {
		t.Fatalf("numeric system configs not applied: %+v", got)
	}
	if len(got.AllowedDownloadHosts) != 2 || got.AllowedDownloadHosts[1] != "cloud.comfy.org" {
		t.Fatalf("allowed hosts not applied: %+v", got.AllowedDownloadHosts)
	}
}
