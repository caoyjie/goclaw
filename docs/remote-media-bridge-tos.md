# Remote Media Bridge with TOS

GoClaw can bridge ComfyUI MCP image outputs into object storage. The gateway downloads the MCP output server-side, uploads it to a TOS-compatible S3 bucket, and appends a safe URL to the final assistant message.

Phase 1 supports Comfy-style image outputs from MCP `get_output` results.

## What It Solves

Some MCP servers return temporary download URLs or structured `download_command` objects. Those URLs are not always safe or usable from the browser chat directly, and the model should not be asked to run `curl` or reconstruct redacted URLs.

The remote media bridge keeps that work in the backend:

1. Detect Comfy MCP output from tool results.
2. Extract an HTTP(S) download URL.
3. Download with host, redirect, timeout, size, and MIME checks.
4. Upload to TOS object storage.
5. Return a public or presigned URL.
6. Append the URL to the assistant answer.

## Object Key Layout

Default prefix:

```text
goclaw-media/
```

Object keys use this structure:

```text
goclaw-media/tenants/{tenant_id}/agents/{agent_id}/sessions/{session_key_hash}/yyyy/mm/dd/{media_id}.{ext}
```

Rules:

- `session_key_hash` is a SHA-256 hash prefix, not the raw session key.
- `agent_id` uses the agent key, not display name.
- File extension comes from verified MIME type.
- Prompt text, usernames, raw Comfy filenames, raw session keys, and signed URL query strings must not appear in object keys.

## Configuration

Use a separate media object-storage config. Do not reuse backup S3 keys.

```json5
{
  media: {
    object_storage: {
      enabled: true,
      provider: "tos",
      bucket: "goclaw-media",
      endpoint: "https://tos-s3-cn-beijing.volces.com",
      region: "cn-beijing",
      prefix: "goclaw-media/",
      access_key_id: "store-in-config-secrets-or-env",
      secret_access_key: "store-in-config-secrets-or-env",
      url_mode: "presigned",
      presign_ttl_seconds: 86400,
      retention_days: 7,
      max_download_bytes: 52428800,
      allowed_download_hosts: [
        "storage.googleapis.com",
        "cloud.comfy.org"
      ]
    }
  }
}
```

Defaults:

- `enabled`: `false`
- `provider`: `tos`
- `prefix`: `goclaw-media/`
- `url_mode`: `presigned`
- `presign_ttl_seconds`: `86400`
- `max_download_bytes`: `52428800`

Secret keys:

- `media.object_storage.access_key_id`
- `media.object_storage.secret_access_key`

These are separate from backup S3 secrets.

## URL Modes

### Private Bucket

Use:

```json5
url_mode: "presigned"
```

The bridge returns a presigned TOS URL. Set `presign_ttl_seconds` based on how long users need access. Common values are 24 hours or 7 days.

### Public Bucket or CDN

Use:

```json5
url_mode: "public",
public_base_url: "https://cdn.example.com"
```

The bridge returns:

```text
{public_base_url}/{object_key}
```

If `public_base_url` is omitted, GoClaw falls back to path-style endpoint URLs:

```text
{endpoint}/{bucket}/{object_key}
```

## Security Model

- Only HTTP and HTTPS download URLs are accepted.
- Cross-host redirects are rejected.
- `allowed_download_hosts` can restrict download hosts.
- Downloads have a byte cap through `max_download_bytes`.
- MIME type is checked against supported media types.
- Object keys reject unsafe path segments.
- Media credentials are independent from backup credentials.
- Remote files are not written to the long-term local `internal/media.Store` directory.

Supported MIME types in Phase 1:

| MIME | Extension | Kind |
|---|---|---|
| `image/png` | `.png` | `image` |
| `image/jpeg` | `.jpg` | `image` |
| `image/webp` | `.webp` | `image` |
| `video/mp4` | `.mp4` | `video` |

Phase 1 only wires Comfy image output behavior. Video MIME support exists at the key/type layer for the next phase.

## Troubleshooting

No URL appears:

- Confirm `media.object_storage.enabled=true`.
- Confirm the Comfy MCP tool result includes a complete URL or `download_command.url`.
- Confirm the agent has MCP grant access to the Comfy server.
- Check gateway logs for `mediaremote.process_failed`.

403 on returned URL:

- Check `url_mode`.
- For presigned URLs, check `presign_ttl_seconds`.
- Confirm bucket, endpoint, region, and credentials match the TOS bucket.
- For public URLs, check bucket policy or CDN origin permissions.

Download blocked:

- Add the Comfy host to `allowed_download_hosts`.
- Check whether the URL redirects to a different host. Cross-host redirects are rejected.
- Confirm the URL uses `http` or `https`.

Object uploaded but not previewing:

- Check the uploaded object content type.
- Confirm the browser can reach TOS or CDN.
- Confirm CORS and public-read/CDN settings if using `url_mode: "public"`.

