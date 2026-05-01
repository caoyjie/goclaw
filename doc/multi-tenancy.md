# Multi-Tenancy in GoClaw

## What is Multi-Tenancy?

GoClaw uses a **shared-schema, row-level isolation** model. A single PostgreSQL instance hosts all tenants in the same tables — every significant table has a `tenant_id UUID` column, and all queries are automatically scoped by that column via `scopeClause()` helpers.

One special tenant exists: the **Master Tenant** (`MasterTenantID = 0193a5b0-7000-7000-8000-000000000001`), which is the default for backward-compatible single-tenant deployments.

---

## What's Isolated Per Tenant?

| Layer | Isolation |
|---|---|
| **Database** | `tenant_id` on 43 tables — agents, sessions, skills, cron jobs, traces, memory, MCP servers, channels, API keys, tasks, etc. UNIQUE constraints (e.g. `session_key`, `agent_key`, `skill_slug`) are per-tenant, not global |
| **Filesystem** | Each non-master tenant gets `dataDir/tenants/{slug}/` and `workspace/tenants/{slug}/` |
| **Auth** | Users must be members of a tenant to access it; checked at WS connect and HTTP auth |
| **API keys** | Scoped to a `tenant_id`, or system-level (`NULL`) with tenant override via `X-GoClaw-Tenant-Id` header |
| **Roles/RBAC** | `owner/admin/operator/member/viewer` per tenant (standard edition only) |

---

## What's Different Between Tenants?

Each tenant is effectively an **isolated workspace** with its own:
- Agents, sessions, skills, cron jobs
- Knowledge graph, memory documents
- MCP server configurations
- Channel integrations (Telegram, Feishu, Zalo, etc.)
- Team tasks
- LLM call traces

Tenants share: the same database instance, the same gateway process, and the same configured LLM providers.

---

## Tenants → Users → Roles → Policies

> **Yes — each tenant has its own set of users, and a single user can have a different role in each tenant they belong to.** That role then drives which RPC methods they can call and which data they can see.

### The relationship in one diagram

```
┌──────────────┐ 1     N ┌────────────────┐ N     1 ┌──────────────┐
│   tenants    ├────────►│ tenant_users   │◄────────┤   user_id    │
│ (workspace)  │         │ (membership +  │         │ (free-form,  │
│              │         │  role mapping) │         │  e.g. tg:42) │
└──────────────┘         └────────────────┘         └──────────────┘
                                  │
                                  ▼
                         role ∈ { owner, admin,
                                  operator, member,
                                  viewer }
```

A row in `tenant_users` is the **N-to-N junction** between a tenant and a user, plus the user's role in that tenant. The composite UNIQUE constraint `(tenant_id, user_id)` means a user has **exactly one role per tenant**, but they can be `admin` in tenant A and `viewer` in tenant B simultaneously.

```sql
-- migrations/000027_tenant_foundation.up.sql
CREATE TABLE tenant_users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id      VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    role         VARCHAR(20) NOT NULL DEFAULT 'member',
    metadata     JSONB NOT NULL DEFAULT '{}',
    ...
    UNIQUE(tenant_id, user_id)   -- one role per (tenant, user)
);
```

### Role hierarchy

Roles are strictly ordered (see `internal/permissions/policy.go::roleLevel`). Higher roles inherit all permissions of lower roles.

| Role | Level | Capabilities |
|---|---|---|
| `owner` | 4 | Tenant management (create/update tenants, manage members) + everything below. Cross-tenant visibility |
| `admin` | 3 | Full RBAC inside the tenant: agent CRUD, channel toggle, API key management, team CRUD, skill update |
| `operator` | 2 | Read + write inside the tenant: chat send/abort, sessions, cron, approvals, pairing |
| `member` | — | Tenant-membership-only role; for runtime RBAC, treated like `viewer` (the gateway role enum has no `member`). Used in `tenant_users.role` to mean "regular user of this tenant" |
| `viewer` | 1 | Read-only: list agents, fetch sessions, view config, read traces |

Note the slight asymmetry: `tenant_users.role` allows `{owner, admin, operator, member, viewer}` (5 values) while the runtime gateway role enum (`permissions.Role`) is `{owner, admin, operator, viewer}` (4 values). `member` is a tenant-membership label only.

### Two role systems (this is subtle but important)

GoClaw has **two distinct role concepts** that operate at different layers:

#### 1. Tenant membership role — stored in `tenant_users.role`

- **Source:** Database row in `tenant_users`
- **Per-tenant:** Yes — same user can have different roles in different tenants
- **Used for:**
  - Tenant management (`tenants.users.add`, `tenants.users.remove`, `tenants.users.list`)
  - Tenant admin checks inside tenant-aware HTTP handlers (e.g. `internal/http/mcp_user_credentials.go` lets a tenant `owner`/`admin` set credentials for any user in their own tenant)
  - The `tenants.mine` RPC returns this role per tenant so the UI shows what the user can do
  - Membership enforcement: connect/auth fails with `TENANT_ACCESS_REVOKED` if the user has no row in `tenant_users` for the requested tenant

#### 2. Session RBAC role — `client.role` (`permissions.Role`)

- **Source:** Determined at WS connect / HTTP auth from the credentials presented
- **Per-session:** Bound to one tenant context for the lifetime of the connection
- **Used for:** Every gateway RPC method goes through `policyEngine.CanAccess(client.role, method)` (see `internal/gateway/router.go::Handle`). HTTP middleware uses `requireAuth(minRole, ...)`.

How it gets assigned (`internal/gateway/router.go::handleConnect`, `internal/http/auth.go::resolveAuth`):

| Credential | Result |
|---|---|
| Gateway token + `user_id` is in `cfg.Gateway.OwnerIDs` | `RoleOwner` (cross-tenant, can scope to any tenant via `tenant_id` param) |
| Gateway token + non-owner user_id | `RoleAdmin`, scoped to the user's tenant (membership required) |
| API key with `tenant_id != NULL` | Role derived from the key's scopes (`RoleFromScopes`), pinned to the key's tenant |
| API key with `tenant_id = NULL` (system-level) | Role from scopes, may scope to any tenant via header (does not become owner) |
| Browser pairing (`X-GoClaw-Sender-Id` / `sender_id` param) | `RoleOperator`, tenant resolved via membership |
| No token configured (dev mode) | `RoleAdmin` (HTTP) / `RoleOperator` (WS), pinned to MasterTenant |
| Token configured but wrong/missing | `RoleViewer` fallback (WS) / 401 (HTTP) |

> **Important:** today, `client.role` is **not** read from `tenant_users.role`. The session role comes from *how* you authenticated, not from your tenant membership row. A user paired in the browser is always `operator` regardless of whether `tenant_users.role` says they are a tenant `admin`. Tenant-level admin powers (e.g. setting MCP credentials for other users) are checked separately by re-reading `tenant_users.role` inside specific handlers.

### API key scopes (the source of API-key roles)

API keys are issued with a set of scopes. `RoleFromScopes()` collapses scopes to a single role:

| Scope | Meaning | Resolves to |
|---|---|---|
| `operator.admin` | Full admin | `RoleAdmin` |
| `operator.write` | Write actions | `RoleOperator` |
| `operator.approvals` | Approve/reject runs | `RoleOperator` |
| `operator.pairing` | Manage pairing | `RoleOperator` |
| `operator.read` | Read-only | `RoleViewer` |
| `operator.provision` | Provision tenants/users | (special; gates `tenants.create`, `tenants.users.add`) |

API keys can additionally be **bound to an owner** (`owner_id`). When set, auth forces `user_id = owner_id` regardless of what the caller sends in the header — preventing identity spoofing through a shared key.

### Per-agent config permissions (a finer-grained policy layer)

Beyond tenant-level RBAC, GoClaw has a per-agent policy table `agent_config_permissions` (`internal/store/config_permission_store.go`). Each row is an allow/deny rule keyed by:

```
(agent_id, scope, config_type, user_id) → "allow" | "deny"
```

- `scope` examples: `agent`, `group:telegram:-100456`, `group:*`, `*`
- `config_type` examples: `heartbeat`, `cron`, `context_files`, `file_writer`, `*`
- Wildcard matching with deny-first evaluation (deny rules win over allow rules)

The most common use: gate file write access in group chats. In a Telegram group, only users granted `file_writer` for the group scope can edit files (`CheckFileWriterPermission` in `config_permission_store.go`).

### The full 5-layer permission stack

From `internal/permissions/policy.go` (top of file):

1. **Gateway Auth** — token/password + scopes (`operator.admin/read/write/approvals/pairing/provision`)
2. **Global Tool Policy** — `tools.allow[]`, `tools.deny[]`, `tools.profile` in config
3. **Per-Agent Policy** — `agents.list[].tools.allow/deny`
4. **Per-Channel/Group Policy** — `channels.*.groups.*.tools.policy` + `agent_config_permissions`
5. **Owner-Only Tools** — `senderIsOwner` check for sensitive tools

Layers 1 & 5 live in `internal/permissions`. Layers 2–4 are computed by `internal/tools/policy.go::PolicyEngine.FilterTools` (a 7-step pipeline that intersects allow lists, applies provider/agent/group overrides, then subtracts deny lists).

---

## Authentication & Tenant Resolution Flow

```
                      ┌──────────────────────────────┐
                      │  Incoming WS connect / HTTP  │
                      │  request with credentials    │
                      └──────────────┬───────────────┘
                                     │
                ┌────────────────────┼────────────────────┐
                ▼                    ▼                    ▼
       Gateway token?         API key bearer?       Pairing sender_id?
                │                    │                    │
                ▼                    ▼                    ▼
       Owner user_id?        Key has tenant_id?    user has membership?
       │           │             │        │             │        │
       Y           N             Y        N             Y        N
       │           │             │        │             │        │
       ▼           ▼             ▼        ▼             ▼        ▼
    OWNER       ADMIN        Pinned to  System-      Operator   401
   (any tenant) (member-     key's      level        (membership
                 ship reqd)  tenant     (any tenant)  enforced)
```

### Tenant hint resolution rules

(`router.go::resolveTenantHint`, `auth.go::resolveTenantHint`)

| Caller | Hint behavior |
|---|---|
| Owner | Free choice — UUID or slug, any tenant. Falls back to `MasterTenant` if unresolved |
| Tenant-bound API key | Hint **ignored**, always pinned to `keyData.TenantID` |
| System-level API key | Hint applied if valid, else `MasterTenant` |
| Anonymous (no `user_id`) + hint | **Denied** (`TENANT_ACCESS_REVOKED`) — cannot scope to a tenant without identity |
| Regular user + hint | Must have a `tenant_users` row for that tenant, else `TENANT_ACCESS_REVOKED` |
| No hint | `MasterTenant` |

The result populates `client.tenantID` / context `TenantIDKey`, which then flows into every store call via `scopeClause()` for row-level isolation.

### Caching

Tenant membership lookups are cached in-process for 30s (`internal/cache/permission_cache.go::tenantRoleTTL`). On membership change, `bus.CacheKindTenantUsers` events invalidate the cache. Removing a user from a tenant also broadcasts `EventTenantAccessRevoked` to force-logout that user's WS sessions.

---

## Use Cases

**1. SaaS product** — You run GoClaw as a hosted service. Each paying customer is a tenant. Customer A's agents and chat history are invisible to Customer B. Different staff at Customer A can have different roles (e.g. CTO is `admin`, support reps are `operator`, dashboards-only stakeholders are `viewer`).

**2. Internal org with departments** — Marketing, Engineering, and Support each get their own tenant. The same person can be `admin` of Engineering and `viewer` of Support — one human, multiple `tenant_users` rows.

**3. Dev / staging separation** — A `production` tenant and a `staging` tenant on the same gateway. SREs are `admin` in both; developers are `admin` in staging but `viewer` in production.

**4. White-label AI product** — You resell AI agents to multiple clients. Each client gets a branded tenant with their own agent configs, memory, and file workspace. The platform owner has `RoleOwner` to provision/manage all tenants; client admins have `admin` only inside their own tenant.

**5. Multi-team platform** — An agency managing AI workflows for multiple brands. Each brand is a tenant with its own cron jobs, sessions, and skill libraries. Account managers have `admin` in the brands they handle and no membership in the others.

---

## Worked Example: One User, Multiple Tenants

User `alice@corp.com` works for an agency that manages two clients on the same GoClaw instance.

```sql
-- alice is admin in acme tenant, viewer in beta tenant
INSERT INTO tenant_users (tenant_id, user_id, role) VALUES
  ('11111111-...', 'alice@corp.com', 'admin'),   -- Acme Corp
  ('22222222-...', 'alice@corp.com', 'viewer');  -- Beta Inc
```

1. Alice connects via WS with her API key (system-level, `tenant_id = NULL`, scopes `[operator.admin]`).
2. She passes `tenant_id: "acme"` in the connect params → `resolveTenantHint` succeeds (membership exists), `client.tenantID = acme`.
3. `client.role = RoleAdmin` (from API key scope), and she sees only Acme data.
4. She opens a second connection with `tenant_id: "beta"` — membership check passes (viewer is still membership), but the global RBAC role is still `RoleAdmin` from her API key scope.
5. **However**, when a *tenant-aware* handler asks "is alice a tenant admin in beta?" (`tenantStore.GetUserRole`), it returns `viewer` — so she cannot, e.g., set MCP credentials on behalf of other users in Beta even though her API key says admin.

This is the dual-role design at work: API key scopes determine API capabilities, while `tenant_users.role` determines tenant-internal authority.

---

## Key Code References

| File | Purpose |
|---|---|
| `migrations/000027_tenant_foundation.up.sql` | Schema: `tenants`, `tenant_users`, `tenant_id` on 43 tables, indexes, per-tenant UNIQUE constraints |
| `internal/permissions/policy.go` | `Role`, `Scope`, `RoleFromScopes`, `MethodRole`, `HasMinRole`, `roleLevel` |
| `internal/store/scope.go` | `QueryScope` — SQL WHERE clause generator |
| `internal/store/context.go` | Context keys + `WithTenantID`, `WithRole`, `IsOwnerRole`, `RoleFromContext` |
| `internal/store/pg/helpers.go` | `scopeClause` / `requireTenantID` / `tenantIDForInsert` |
| `internal/store/tenant_store.go` | `TenantStore` interface, `MasterTenantID`, role + status constants |
| `internal/store/pg/tenant_store.go` | PG impl: CRUD + `GetUserRole`, `ListUserTenants`, `ResolveUserTenant` |
| `internal/store/config_permission_store.go` | Per-(agent, scope, configType, user) allow/deny + `CheckFileWriterPermission` |
| `internal/cache/permission_cache.go` | 30s TTL cache for tenant role lookups |
| `internal/gateway/router.go` | WS `connect`: tenant + role resolution, membership enforcement, `TENANT_ACCESS_REVOKED` |
| `internal/http/auth.go` | HTTP auth: token/API-key/pairing → `(role, tenantID, tenantSlug)` |
| `internal/http/tenant_auth_helpers.go` | `requireTenantMembership` helpers shared by HTTP handlers |
| `internal/http/mcp_user_credentials.go` | Example of using `tenant_users.role` for tenant-internal admin checks |
| `internal/config/tenant_paths.go` | Filesystem path isolation per tenant |
| `internal/http/files.go` | File serving: tenant-scoped path enforcement |
| `internal/gateway/methods/tenants.go` | RPC: `tenants.list/get/create/update`, `tenants.users.{list,add,remove}`, `tenants.mine` |
| `internal/gateway/methods/access.go` | `canSeeAll(role, ownerIDs, userID)` helper used by session/cron/chat handlers |
| `internal/tools/policy.go` | Tool-level policy engine (layers 2–4 of the 5-layer stack) |

---

## Notes

- The **Lite (desktop) edition** does not support multi-tenancy — single-tenant only (`internal/edition/edition.go`). Tools `skill_manage` and `publish_skill` are not registered, and `team_tasks` actions are limited (see `internal/tools/team_action_policy.go::LiteTeamPolicy`).
- `IsCrossTenant(ctx)` is deprecated; new code uses `IsOwnerRole(ctx)` for permission guards and `scopeClause` for SQL.
- SQLite edition uses `?` placeholders; PG edition uses `$N` — both share the same contract via `internal/store/sqlitestore/scope.go`.
- Removing a user from a tenant emits `EventTenantAccessRevoked` over the message bus, which terminates that user's active WS sessions for the affected tenant.
- The `TENANT_ACCESS_REVOKED` error code is the single uniform signal returned by both WS connect and HTTP auth when a user attempts to access a tenant they are not a member of.
