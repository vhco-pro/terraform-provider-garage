---
status: done
status_description: "Fully implemented. resource_admin_token.go (CRUD with SecretToken at creation only, UseStateForUnknown), datasource_admin_token.go (by ID or current=true), datasource_admin_tokens.go (list all). Build and lint clean."
description: "Implement garage_admin_token resource and admin token data sources"
author: "garage-operator team"
goal: "Admin token lifecycle management and discovery via Terraform"
priority: medium
created: 2026-04-06
---

# Plan: Admin Token Resources

This plan implements `garage_admin_token` resource and the `garage_admin_token` / `garage_admin_tokens` data sources. Admin tokens allow programmatic management of who can access the Garage Admin API.

## Context

Admin token management is completely absent from the [existing provider](../research/terraform-provider-analysis.md). For multi-tenant setups and automation, managing admin API access declaratively is important. The Garage API v2 added full admin token CRUD (6 operations).

Depends on: [Plan 01 — Project Scaffold](01-project-scaffold.md)

## Scope

### In Scope

- `garage_admin_token` resource — Full CRUD lifecycle
- `garage_admin_token` data source — Lookup by ID or get current token info
- `garage_admin_tokens` data source — List all tokens

### Out of Scope

- Token rotation workflows (belongs in Terraform modules)
- `GetCurrentAdminTokenInfo` as a separate data source (merged into `garage_admin_token` data source with `current = true` flag)

## Design

### API Operations Used

| Operation | Used By |
|---|---|
| `CreateAdminToken` | `garage_admin_token` Create |
| `GetAdminTokenInfo` | `garage_admin_token` Read + data source |
| `UpdateAdminToken` | `garage_admin_token` Update |
| `DeleteAdminToken` | `garage_admin_token` Delete |
| `GetCurrentAdminTokenInfo` | `garage_admin_token` data source (with `current = true`) |
| `ListAdminTokens` | `garage_admin_tokens` data source |

### Resource: `garage_admin_token`

#### Schema

| Attribute | Type | Req/Opt/Comp | Plan Modifier | Validator | Description |
|---|---|---|---|---|---|
| `id` | String | Computed | `UseStateForUnknown` | — | Token identifier (also a prefix of the full bearer token) |
| `name` | String | Optional | — | `LengthBetween(1, 255)` | Human-friendly name |
| `secret_token` | String, Sensitive | Computed | `UseStateForUnknown` | — | Full bearer token value (only available at creation — the API field is `secretToken`) |
| `expiration` | String | Optional | — | RFC 3339 format validator | Token expiration timestamp |
| `never_expires` | Bool | Optional | — | — | Set to `true` to explicitly make the token never expire (conflicts with `expiration`) |
| `scope` | List of String | Optional | — | Each element validated against known endpoint names or `"*"` | Permitted admin API endpoint names (e.g., `"GetClusterStatus"`, `"CreateBucket"`, `"*"`) |
| `expired` | Bool | Computed | — | — | Whether this token has expired (set by Garage). Informational only — does not trigger resource replacement when token expires between applies. |
| `created` | String | Computed | — | — | Creation timestamp |

#### Scope Validator: Canonical Operation List

The scope validator accepts the special value `"*"` (all endpoints) or any of the 49 `operationId` values from the Garage Admin API v2.2.0 OpenAPI spec:

**Admin Tokens (6):** `CreateAdminToken`, `DeleteAdminToken`, `GetAdminTokenInfo`, `GetCurrentAdminTokenInfo`, `ListAdminTokens`, `UpdateAdminToken`
**Buckets (7):** `CreateBucket`, `DeleteBucket`, `GetBucketInfo`, `ListBuckets`, `UpdateBucket`, `CleanupIncompleteUploads`, `InspectObject`
**Bucket Aliases (2):** `AddBucketAlias`, `RemoveBucketAlias`
**Keys (6):** `CreateKey`, `DeleteKey`, `GetKeyInfo`, `ImportKey`, `ListKeys`, `UpdateKey`
**Permissions (2):** `AllowBucketKey`, `DenyBucketKey`
**Layout (6):** `ApplyClusterLayout`, `GetClusterLayout`, `GetClusterLayoutHistory`, `RevertClusterLayout`, `UpdateClusterLayout`, `ClusterLayoutSkipDeadNodes`
**Cluster (4):** `GetClusterHealth`, `GetClusterStatus`, `GetClusterStatistics`, `PreviewClusterLayoutChanges`
**Connectivity (1):** `ConnectClusterNodes`
**Nodes (4):** `CreateMetadataSnapshot`, `GetNodeInfo`, `GetNodeStatistics`, `LaunchRepairOperation`
**Blocks (4):** `GetBlockInfo`, `ListBlockErrors`, `PurgeBlocks`, `RetryBlockResync`
**Workers (4):** `GetWorkerInfo`, `GetWorkerVariable`, `ListWorkers`, `SetWorkerVariable`
**Special (3):** `CheckDomain`, `Health`, `Metrics`

> **Versioning note:** This list is extracted from the `operationId` fields in the OpenAPI spec v2.2.0. If Garage adds new operations in a future version, the provider's validator must be updated. Add a code comment pointing to `internal/garage/openapi/spec.json` as the source of truth.

#### CRUD Implementation

**Create:**
1. Call `CreateAdminToken` with optional `name`, `expiration`, `never_expires`, `scope`
2. Store returned `id` and `secretToken` as `secret_token` (secret only available now)
3. Read back full state including `expired` flag

**Read:**
1. Call `GetAdminTokenInfo` by ID
2. If 404 → remove from state
3. Map all fields except `secret_token` (not returned after creation — preserved via `UseStateForUnknown`)
4. Map `expired` boolean from response — informational only, does not trigger resource changes

**Update:**
1. Call `UpdateAdminToken` with changed fields (`name`, `expiration`, `never_expires`, `scope`)
2. Read back state

**Delete:**
1. Call `DeleteAdminToken` by ID
2. If 404 → already deleted, no error

**Import:**
- By token ID
- `secret_token` will be null after import (documented limitation)
- Import emits warning diagnostic: "secret_token is not available after import"

#### Example HCL

```hcl
resource "garage_admin_token" "ci" {
  name       = "ci-pipeline"
  expiration = "2027-01-01T00:00:00Z"
  scope      = ["GetClusterStatus", "GetClusterHealth", "ListBuckets"]
}

resource "garage_admin_token" "superadmin" {
  name          = "full-access"
  never_expires = true
  scope         = ["*"]
}

output "ci_token" {
  value     = garage_admin_token.ci.secret_token
  sensitive = true
}
```

---

### Data Source: `garage_admin_token`

#### Schema

| Attribute | Type | Req/Opt/Comp | Description |
|---|---|---|---|
| `id` | String | Optional | Token ID (one of `id` or `current` required) |
| `current` | Bool | Optional | If true, returns info about the token being used for API auth |
| `name` | String | Computed | Token name |
| `expiration` | String | Computed | Expiration timestamp |
| `scope` | List of String | Computed | Permitted API endpoint names |
| `expired` | Bool | Computed | Whether token has expired |
| `created` | String | Computed | Creation timestamp |

**Validation:** Exactly one of `id` or `current = true` must be specified. Enforced with `ExactlyOneOf` path expression validator:

```go
"id": schema.StringAttribute{
    Optional: true,
    Validators: []validator.String{
        stringvalidator.ExactlyOneOf(path.MatchRoot("current")),
    },
},
```

**Implementation:**
- If `id` provided → `GetAdminTokenInfo`
- If `current = true` → `GetCurrentAdminTokenInfo`

---

### Data Source: `garage_admin_tokens`

#### Schema

| Attribute | Type | Description |
|---|---|---|
| `tokens` | List of Object | All admin tokens |
| `tokens.*.id` | String | Token ID |
| `tokens.*.name` | String | Token name |
| `tokens.*.expiration` | String | Expiration timestamp |
| `tokens.*.scope` | List of String | Permitted endpoint names |
| `tokens.*.expired` | Bool | Whether token has expired |
| `tokens.*.created` | String | Token creation timestamp |

**Implementation:** Call `ListAdminTokens`, map response.

## Acceptance Criteria

### `garage_admin_token` Resource

- [ ] Create returns `id` and `secret_token` (`secret_token` is sensitive)
- [ ] `secret_token` is preserved across reads via `UseStateForUnknown`
- [ ] Create with `name` and `expiration` sets both in Garage
- [ ] Create with `never_expires = true` creates a non-expiring token
- [ ] `expiration` and `never_expires` are mutually exclusive (validation error if both set)
- [ ] Update `name` persists to Garage (verifiable via `GetAdminTokenInfo`)
- [ ] Update `expiration` updates the expiration timestamp
- [ ] Update `scope` changes permitted endpoints
- [ ] Delete calls `DeleteAdminToken`
- [ ] Import by ID restores all fields except `secret_token`
- [ ] Import emits warning diagnostic about missing `secret_token`
- [ ] `expiration` validated as RFC 3339 format
- [ ] `scope` elements validated (each must be a known Garage endpoint name or `"*"`)
- [ ] `expired` computed attribute reflects current expiration state

### Data Sources

- [ ] `garage_admin_token` with `id` returns token info
- [ ] `garage_admin_token` with `current = true` returns current token info
- [ ] `garage_admin_token` without `id` or `current` returns validation error
- [ ] `garage_admin_tokens` returns list of all tokens
- [ ] `garage_admin_tokens` returns empty list when only bootstrap token exists

## Implementation Phases

### Phase 1: Admin Token Resource

- [ ] Create `internal/provider/resource_admin_token.go` with schema
- [ ] Implement CRUD with token preservation
- [ ] Implement ImportState
- [ ] Add validators (name length, expiration RFC 3339)
- [ ] Create examples

### Phase 2: Admin Token Data Sources

- [ ] Create `internal/provider/datasource_admin_token.go` with ID/current lookup
- [ ] Create `internal/provider/datasource_admin_tokens.go` with list
- [ ] Add validation (exactly one of id/current)
- [ ] Create examples

### Phase 3: Register and Verify

- [ ] Register in `provider.go`
- [ ] Verify `make build` succeeds

## Test Plan

See [Plan 06 — Testing Strategy](06-testing.md) for complete test definitions.

| Acceptance Criterion | Test ID | Test Type | Location |
|---|---|---|---|
| Admin token create (id + secret_token) | T1 | Acceptance | `resource_admin_token_test.go` |
| Admin token update name | T2 | Acceptance | `resource_admin_token_test.go` |
| Admin token update expiration | T3 | Acceptance | `resource_admin_token_test.go` |
| Admin token update scope | T4 | Acceptance | `resource_admin_token_test.go` |
| Admin token never_expires | T5 | Acceptance | `resource_admin_token_test.go` |
| Admin token import (secret null) | T6 | Acceptance | `resource_admin_token_test.go` |
| Admin token secret preserved | T7 | Acceptance | `resource_admin_token_test.go` |
| Admin token delete | T8 | Acceptance | `resource_admin_token_test.go` |
| Admin token disappears externally | T9 | Acceptance | `resource_admin_token_test.go` |
| Expiration + never_expires conflict | T10 | Acceptance | `resource_admin_token_test.go` |
| Invalid expiration format | T11 | Acceptance | `resource_admin_token_test.go` |
| Invalid scope endpoint name | T12 | Acceptance | `resource_admin_token_test.go` |
| Scope validation (known endpoints) | U28-U29 | Unit | `internal/provider/validators_test.go` |
| Scope validation (invalid inputs) | U30-U31 | Unit | `internal/provider/validators_test.go` |
| RFC 3339 validator | U36 | Unit | `internal/provider/validators_test.go` |
| Data source by ID | D15 | Acceptance | `datasource_admin_token_test.go` |
| Data source current token | D16 | Acceptance | `datasource_admin_token_test.go` |
| Data source missing id+current | D17 | Acceptance | `datasource_admin_token_test.go` |
| List all tokens | D18 | Acceptance | `datasource_admin_tokens_test.go` |

## Resolved Questions

1. **Admin token scopes** — Scopes are **Garage Admin API endpoint operation names** (e.g., `"GetClusterStatus"`, `"CreateBucket"`, `"ListKeys"`). The special value `"*"` grants access to all endpoints. **Warning:** Granting `"CreateAdminToken"` or `"UpdateAdminToken"` allows privilege escalation (equivalent to `"*"`). This is documented in the provider docs.
2. **Token expiration behavior** — The API returns `expired: true` boolean on expired tokens. They remain queryable via `GetAdminTokenInfo` and `ListAdminTokens`. The provider maps this as a computed `expired` attribute. No special handling needed — it's informational.
3. **Bootstrap token** — The bootstrap admin token (configured in `garage.toml`) does NOT appear in `ListAdminTokens` and cannot be managed via the API. It's only usable via `GetCurrentAdminTokenInfo` if it's the token being used for auth. Treat it as outside provider scope — it's a Garage server config concern.
