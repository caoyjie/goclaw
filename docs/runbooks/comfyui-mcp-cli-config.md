# Configure ComfyUI MCP for an Agent

This runbook documents how to configure the ComfyUI MCP server for GoClaw and grant an agent full access to that MCP server.

GoClaw currently has `goclaw agent` and `goclaw config` CLI commands, but it does not have a dedicated `goclaw mcp` subcommand. MCP server configuration is managed through the Gateway HTTP API. Use the `goclaw` CLI to discover agents and validate runtime config, then use the HTTP API for MCP server and grant operations.

## Current Comfy Cloud MCP Shape

The current ComfyUI MCP server should use this shape:

```json
{
  "name": "comfy-cloud",
  "display_name": "Comfy Cloud",
  "transport": "streamable-http",
  "url": "https://cloud.comfy.org/mcp",
  "tool_prefix": "mcp_comfy_cloud",
  "timeout_sec": 120,
  "settings": {
    "oauth": {
      "auth_type": "oauth",
      "scope": "comfy-mcp:tools:call"
    }
  },
  "enabled": true
}
```

Important fields:

- `transport`: use `streamable-http` for Comfy Cloud MCP.
- `url`: Comfy Cloud MCP endpoint.
- `tool_prefix`: GoClaw prefixes registered MCP tools with this value.
- `timeout_sec`: image/video generation can be slow, so use a larger timeout such as `120`.
- `settings.oauth`: Comfy Cloud MCP requires OAuth.

## 1. Get Gateway Server and Token

For local Docker deployment, the Gateway usually listens on `http://localhost:18790`.

Inside the running container, the token is available as `GOCLAW_GATEWAY_TOKEN`:

```bash
TOKEN="$(docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.browser.yml \
  -f docker-compose.otel.yml \
  -f docker-compose.redis.yml \
  -f docker-compose.sandbox-ceo.yml \
  exec -T goclaw sh -lc 'printf %s "$GOCLAW_GATEWAY_TOKEN"')"

SERVER="http://localhost:18790"
```

Do not paste the token into docs, logs, commits, or chat messages.

## 2. Confirm the Agent with GoClaw CLI

Use the GoClaw CLI to list agents:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.browser.yml \
  -f docker-compose.otel.yml \
  -f docker-compose.redis.yml \
  -f docker-compose.sandbox-ceo.yml \
  exec -T goclaw ./goclaw agent list \
    --server "$SERVER" \
    --token "$TOKEN"
```

For the current local deployment, the expected agent is:

```text
agent_key: ceo-assistant
display:   1号助手
type:      predefined
status:    active
```

The HTTP MCP grant API needs the agent UUID, not `agent_key`. If needed, query it from PostgreSQL:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.browser.yml \
  -f docker-compose.otel.yml \
  -f docker-compose.redis.yml \
  -f docker-compose.sandbox-ceo.yml \
  exec -T postgres psql -U goclaw -d goclaw \
  -c "select id, agent_key, display_name, status from agents order by agent_key;"
```

Current known local value:

```text
ceo-assistant = 019e9168-7aa3-7cf4-a38a-f0bbcbb57d69
```

## 3. Create or Update the Comfy MCP Server

First list existing MCP servers:

```bash
curl -sS "$SERVER/v1/mcp/servers" \
  -H "Authorization: Bearer $TOKEN"
```

If `comfy-cloud` does not exist, create it:

```bash
curl -sS -X POST "$SERVER/v1/mcp/servers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "comfy-cloud",
    "display_name": "Comfy Cloud",
    "transport": "streamable-http",
    "url": "https://cloud.comfy.org/mcp",
    "tool_prefix": "mcp_comfy_cloud",
    "timeout_sec": 120,
    "settings": {
      "oauth": {
        "auth_type": "oauth",
        "scope": "comfy-mcp:tools:call"
      }
    },
    "enabled": true
  }'
```

If it already exists, keep its `id` and update it:

```bash
SERVER_ID="<mcp_server_uuid>"

curl -sS -X PUT "$SERVER/v1/mcp/servers/$SERVER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Comfy Cloud",
    "transport": "streamable-http",
    "url": "https://cloud.comfy.org/mcp",
    "tool_prefix": "mcp_comfy_cloud",
    "timeout_sec": 120,
    "settings": {
      "oauth": {
        "auth_type": "oauth",
        "scope": "comfy-mcp:tools:call"
      }
    },
    "enabled": true
  }'
```

Current known local value:

```text
comfy-cloud = 019f1290-6ecd-79ac-9e30-d0d14dbdfdb4
```

## 4. Authorize Comfy OAuth

Check OAuth status:

```bash
curl -sS "$SERVER/v1/mcp/oauth/status/$SERVER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

If the token is missing or expired and refresh does not recover it, start a new OAuth flow:

```bash
curl -sS -X POST "$SERVER/v1/mcp/oauth/start" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"server_id\": \"$SERVER_ID\",
    \"mcp_url\": \"https://cloud.comfy.org/mcp\"
  }"
```

Open the returned authorization URL in a browser and finish the Comfy Cloud login. The callback endpoint is served by GoClaw at:

```text
/v1/mcp/oauth/callback
```

For local Docker use, make sure Comfy Cloud can reach the callback URL if the OAuth provider requires a public redirect URL. If local callback cannot be reached externally, expose GoClaw through your existing Cloudflare Tunnel or public Gateway URL, then start OAuth from that public URL.

## 5. Grant the Agent Full MCP Permission

Grant full access by omitting `tool_allow` and `tool_deny`.

```bash
AGENT_ID="019e9168-7aa3-7cf4-a38a-f0bbcbb57d69"
SERVER_ID="019f1290-6ecd-79ac-9e30-d0d14dbdfdb4"

curl -sS -X POST "$SERVER/v1/mcp/servers/$SERVER_ID/grants/agent" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"agent_id\": \"$AGENT_ID\"
  }"
```

Full permission means:

- `mcp_agent_grants.enabled = true`
- `tool_allow` is `NULL` or omitted, which means all tools are allowed
- `tool_deny` is `NULL` or omitted, which means no tools are denied

Verify from the API:

```bash
curl -sS "$SERVER/v1/mcp/grants/agent/$AGENT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

Or verify from PostgreSQL:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.browser.yml \
  -f docker-compose.otel.yml \
  -f docker-compose.redis.yml \
  -f docker-compose.sandbox-ceo.yml \
  exec -T postgres psql -U goclaw -d goclaw \
  -c "select ms.name as server_name, a.agent_key, mag.enabled, mag.tool_allow, mag.tool_deny
      from mcp_agent_grants mag
      join mcp_servers ms on ms.id = mag.server_id
      join agents a on a.id = mag.agent_id
      order by ms.name, a.agent_key;"
```

Expected result for full access:

```text
server_name  | agent_key     | enabled | tool_allow | tool_deny
-------------+---------------+---------+------------+----------
comfy-cloud  | ceo-assistant | t       |            |
```

## 6. Reconnect and Check Tools

Reconnect the MCP server after changing config or credentials:

```bash
curl -sS -X POST "$SERVER/v1/mcp/servers/$SERVER_ID/reconnect" \
  -H "Authorization: Bearer $TOKEN"
```

List discovered tools:

```bash
curl -sS "$SERVER/v1/mcp/servers/$SERVER_ID/tools" \
  -H "Authorization: Bearer $TOKEN"
```

The agent should then see Comfy MCP tools with the configured prefix, for example names beginning with:

```text
mcp_comfy_cloud_
```

## 7. Test from the Agent CLI

Send a one-shot chat through the GoClaw CLI:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.browser.yml \
  -f docker-compose.otel.yml \
  -f docker-compose.redis.yml \
  -f docker-compose.sandbox-ceo.yml \
  exec -T goclaw ./goclaw agent chat ceo-assistant \
    --server "$SERVER" \
    --token "$TOKEN" \
    "Use the Comfy Cloud MCP tools to list available generation capabilities."
```

If the agent cannot see or call the tools, check in this order:

1. `mcp_servers.enabled = true`
2. Comfy OAuth status is valid
3. `mcp_agent_grants.enabled = true`
4. `tool_allow` and `tool_deny` are empty for full access
5. `/v1/mcp/servers/{id}/tools` returns Comfy tools
6. Gateway logs do not show OAuth, SSRF, timeout, or MCP connection errors

