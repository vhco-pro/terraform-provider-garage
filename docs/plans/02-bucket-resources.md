---
status: done
status_description: "Fully implemented. resource_bucket.go (two-step create for quotas/website), resource_bucket_alias.go (global/local via BucketAliasEnum union), datasource_bucket.go (by ID or global_alias), datasource_buckets.go (list all). validators.go with regexpBucketAlias. Build and lint clean."
description: "Implement garage_bucket resource, garage_bucket_alias resource, and bucket data sources"
author: "garage-operator team"
goal: "Full bucket lifecycle management via Terraform"
priority: high
created: 2026-04-06
---

# Plan: Bucket Resources and Data Sources

This plan implements everything related to Garage S3 buckets: the `garage_bucket` resource (CRUD + quotas + website), the `garage_bucket_alias` resource (global and local aliases), and the `garage_bucket` / `garage_buckets` data sources.

## Context

Buckets are the primary user-facing resource in Garage and the most important resource in the provider. The [existing provider](../research/terraform-provider-analysis.md) has a `garage_bucket` resource with a website state asymmetry bug (flat bools vs nested API object) and no alias management support. We fix this with a properly nested schema and a dedicated alias resource.

Depends on: [Plan 01 — Project Scaffold](01-project-scaffold.md)

## Scope

### In Scope

- `garage_bucket` resource — Create, Read, Update, Delete, Import
- `garage_bucket_alias` resource — Add/Remove global and local aliases
- `garage_bucket` data source — Lookup by ID or global alias
- `garage_buckets` data source — List all buckets
- Validators for all user-facing attributes
- Examples for all resources and data sources

### Out of Scope

- Bucket permissions (Plan 03 as part of key resources)
- `CleanupIncompleteUploads` (imperative, not declarative)

## Design

### API Operations Used

| Operation | Used By |
|---|---|
| `CreateBucket` | `garage_bucket` Create |
| `GetBucketInfo` | `garage_bucket` Read + `garage_bucket` data source |
| `UpdateBucket` | `garage_bucket` Update (quotas, website) |
| `DeleteBucket` | `garage_bucket` Delete |
| `ListBuckets` | `garage_buckets` data source |
| `AddBucketAlias` | `garage_bucket` Create (initial alias) + `garage_bucket_alias` Create |
| `RemoveBucketAlias` | `garage_bucket_alias` Delete |

### Resource: `garage_bucket`

#### Schema

| Attribute | Type | Required/Optional/Computed | Plan Modifier | Validator | Description |
|---|---|---|---|---|---|
| `id` | String | Computed | `UseStateForUnknown` | — | Garage bucket ID (hex UUID) |
| `created` | String | Computed | — | — | Bucket creation timestamp (RFC 3339) |
| `global_alias` | String | Required | `RequiresReplace` | `LengthBetween(1, 63)`, `RegexMatches(^[a-z0-9][a-z0-9.-]*$)` | Primary bucket name |
| `website` | SingleNestedAttribute, Optional | — | — | — | Website hosting config block |
| `website.enabled` | Bool | Required | — | — | Enable/disable website hosting |
| `website.index_document` | String | Optional | — | — | Index document (e.g., `index.html`). **Required when `enabled = true`** (provider-imposed constraint for UX safety; the API schema allows null but Garage rejects it at runtime). |
| `website.error_document` | String | Optional | — | — | Error document (e.g., `error.html`) |
| `quotas` | SingleNestedAttribute, Optional | — | — | — | Bucket quotas block |
| `quotas.max_size` | Int64 | Optional | — | `AtLeast(0)` | Max total size in bytes |
| `quotas.max_objects` | Int64 | Optional | — | `AtLeast(0)` | Max number of objects |
| `objects` | Int64 | Computed | — | — | Current object count |
| `bytes` | Int64 | Computed | — | — | Current bucket size in bytes |
| `unfinished_uploads` | Int64 | Computed | — | — | Pending multipart uploads (legacy) |
| `unfinished_multipart_uploads` | Int64 | Computed | — | — | Pending multipart upload count |
| `unfinished_multipart_upload_parts` | Int64 | Computed | — | — | Parts in pending multipart uploads |
| `unfinished_multipart_upload_bytes` | Int64 | Computed | — | — | Bytes used by pending multipart uploads |
| `global_aliases` | List of String | Computed | — | — | All global aliases (read-only) |

**Design decisions:**

1. **`website` as nested object** — The Garage API has a read/write asymmetry. On **read**, `GetBucketInfoResponse` returns TWO fields: `websiteAccess` (boolean) and `websiteConfig` (nullable object `{indexDocument, errorDocument}`). On **write**, `UpdateBucketRequestBody.websiteAccess` is a **nested object** (`{enabled, indexDocument, errorDocument}`). We normalize this in the schema: when reading, if `websiteAccess` is `true` we populate `website.enabled = true` and fill `index_document`/`error_document` from `websiteConfig` (which IS returned by the API, contrary to earlier assumptions). When `website` block is absent → hosting is disabled.

2. **`global_alias` as required + `RequiresReplace`** — Every bucket must have a primary name. Changing it destroys and recreates (Garage doesn't support renaming). Additional aliases are managed via `garage_bucket_alias`.

3. **Computed `global_aliases`** — Read-only view of all global aliases including ones managed by `garage_bucket_alias` resources. Avoids state conflicts. Note: `GetBucketInfoResponse` does NOT have a top-level `localAliases` field — local aliases are only available inside `keys[].bucketLocalAliases` (as string arrays). The `ListBuckets` response DOES have `localAliases` at the top level. The bucket resource does not expose `local_aliases` to avoid this inconsistency; local aliases are visible via `garage_bucket_alias` resources and the `garage_buckets` data source.

4. **Create is two-step** — `CreateBucket` only accepts `globalAlias`/`localAlias` (no quotas, no website). If quotas or website are specified, a follow-up `UpdateBucket` call is required. If `UpdateBucket` fails, the bucket exists but without the desired settings. The provider should read back state and report the partial creation error, so the next `terraform apply` can retry the `UpdateBucket`.

#### CRUD Implementation

**Create:**
1. Call `CreateBucket` with `globalAlias` set to `global_alias`
2. If `quotas` or `website` specified, call `UpdateBucket` to set them
3. Read back full state via `GetBucketInfo`

**Read:**
1. Call `GetBucketInfo` by ID
2. If 404 → `resp.State.RemoveResource(ctx)` (bucket deleted externally)
3. Map response to state: quotas, website config, aliases, usage stats

**Update:**
1. Build `UpdateBucketRequestBody` with changed quotas and/or website config
2. Call `UpdateBucket` — **Quota constraint:** both `maxSize` and `maxObjects` must be specified together (or both set to `null` to remove). It is not possible to change only one of the two quotas.
3. Read back full state

**Delete:**
1. Call `DeleteBucket` with bucket ID (via `?id=<bucket_id>` query param)
2. If 400 (bucket not empty) → `AddError("bucket must be empty before deletion")` — Note: Garage returns **400**, not 409
3. If 404 → already deleted, no error

**Import:**
- `ImportStatePassthroughID` — import by bucket ID
- Read automatically fills all computed and optional+computed attributes

#### Example HCL

```hcl
resource "garage_bucket" "website" {
  global_alias = "my-website"

  website = {
    enabled        = true
    index_document = "index.html"
    error_document = "error.html"
  }

  quotas = {
    max_size    = 10737418240  # 10 GB
    max_objects = 100000
  }
}
```

---

### Resource: `garage_bucket_alias`

Manages additional global or local aliases for a bucket. Separate from the bucket resource to follow "one resource per API object" principle and support incremental alias management.

#### Schema

| Attribute | Type | Required/Optional/Computed | Plan Modifier | Validator | Description |
|---|---|---|---|---|---|
| `id` | String | Computed | `UseStateForUnknown` | — | Composite: `{bucket_id}:{alias_type}:{alias_name}` or `{bucket_id}:{alias_type}:{key_id}:{alias_name}` |
| `bucket_id` | String | Required | `RequiresReplace` | `LengthAtLeast(1)` | Target bucket ID |
| `alias_type` | String | Required | `RequiresReplace` | `OneOf("global", "local")` | Alias type |
| `name` | String | Required | `RequiresReplace` | `LengthBetween(1, 63)` | Alias name |
| `access_key_id` | String | Optional | `RequiresReplace` | `RegexMatches(^GK[0-9a-f]{24}$)` | Required for local aliases — the key this alias is scoped to |

**Validation:** If `alias_type = "local"`, then `access_key_id` is required. If `alias_type = "global"`, then `access_key_id` must not be set.

#### CRUD Implementation

**Create:**
1. Call `AddBucketAlias` with global or local alias payload
2. Construct composite ID

**Read:**
1. Call `GetBucketInfo` for the bucket
2. For global aliases: search `globalAliases` for our alias name
3. For local aliases: iterate `keys[]` array, search each key's `bucketLocalAliases` (string array) for our alias name scoped to our `access_key_id`
4. If not found → `resp.State.RemoveResource(ctx)`

**Delete:**
1. Call `RemoveBucketAlias` with global or local alias payload

**Update:** Not applicable — all fields use `RequiresReplace`

**Import:** By composite ID `{bucket_id}:{alias_type}:{alias_name}` (or `{bucket_id}:local:{key_id}:{alias_name}`)

#### Example HCL

```hcl
# Additional global alias for an existing bucket
resource "garage_bucket_alias" "cdn" {
  bucket_id  = garage_bucket.website.id
  alias_type = "global"
  name       = "cdn-assets"
}

# Local alias scoped to a specific key
resource "garage_bucket_alias" "app_local" {
  bucket_id     = garage_bucket.data.id
  alias_type    = "local"
  name          = "app-data"
  access_key_id = garage_key.app.id
}
```

---

### Data Source: `garage_bucket`

#### Schema

| Attribute | Type | Required/Optional/Computed | Description |
|---|---|---|---|
| `id` | String | Optional | Bucket ID (one of `id` or `global_alias` required) |
| `global_alias` | String | Optional | Bucket alias (one of `id` or `global_alias` required) |
| `global_aliases` | List of String | Computed | All global aliases |
| `local_aliases` | List of Object | Computed | All local aliases with key info |
| `website` | Object | Computed | Website hosting config |
| `quotas` | Object | Computed | Bucket quotas |
| `objects` | Int64 | Computed | Current object count |
| `bytes` | Int64 | Computed | Current bucket size |
| `unfinished_uploads` | Int64 | Computed | Pending multipart uploads |
| `keys` | List of Object | Computed | Keys with permissions on this bucket |

**Validation:** Exactly one of `id` or `global_alias` must be specified.

**Implementation:** 
- If `id` provided → `GetBucketInfo?id=<id>`
- If `global_alias` provided → `GetBucketInfo?globalAlias=<alias>`

#### Example HCL

```hcl
data "garage_bucket" "existing" {
  global_alias = "production-data"
}

output "bucket_size" {
  value = data.garage_bucket.existing.bytes
}
```

---

### Data Source: `garage_buckets`

#### Schema

| Attribute | Type | Description |
|---|---|---|
| `buckets` | List of Object | All buckets |
| `buckets.*.id` | String | Bucket ID |
| `buckets.*.global_aliases` | List of String | Bucket aliases |
| `buckets.*.local_aliases` | List of Object | Local aliases (from `ListBuckets` response) |
| `buckets.*.local_aliases.*.access_key_id` | String | Key ID the local alias is scoped to |
| `buckets.*.local_aliases.*.alias` | String | Local alias name |
| `buckets.*.created` | String | Bucket creation timestamp (RFC 3339) |

**Implementation:** Call `ListBuckets`, map response. Note: `ListBuckets` returns `localAliases` at the top level (unlike `GetBucketInfo` which only has them inside `keys[]`).

#### Example HCL

```hcl
data "garage_buckets" "all" {}

output "bucket_count" {
  value = length(data.garage_buckets.all.buckets)
}
```

## Acceptance Criteria

### `garage_bucket` Resource

- [ ] Create with `global_alias` creates bucket with alias visible via `GetBucketInfo`
- [ ] Create with `quotas.max_size` persists quota to Garage (readable via `GetBucketInfo`)
- [ ] Create with `website` block enables website access and sets index/error documents via `UpdateBucket`
- [ ] Read returns current quota usage (`objects`, `bytes`, `unfinished_uploads`)
- [ ] Update quotas persists new values to Garage (verifiable via read)
- [ ] Update website from enabled to disabled persists to Garage
- [ ] Delete empty bucket succeeds
- [ ] Delete non-empty bucket returns error diagnostic containing "bucket must be empty before deletion"
- [ ] Import by ID restores full state including quotas and website config
- [ ] Changing `global_alias` triggers destroy+recreate (not in-place update)
- [ ] `global_aliases` computed attribute reflects all aliases including ones from `garage_bucket_alias`
- [ ] Partial create failure (CreateBucket OK, UpdateBucket fails) stores bucket ID in state so next apply can retry

### `garage_bucket_alias` Resource

- [ ] Create global alias calls `AddBucketAlias` with global alias payload
- [ ] Create local alias calls `AddBucketAlias` with local alias payload including `accessKeyId`
- [ ] Create alias that already exists on the same bucket succeeds idempotently (no error)
- [ ] Create alias that exists on a different bucket returns clear error
- [ ] Delete global alias calls `RemoveBucketAlias`
- [ ] Read verifies alias still exists on bucket; removes from state if not
- [ ] Validation: `alias_type = "local"` without `access_key_id` returns error at plan time
- [ ] Validation: `alias_type = "global"` with `access_key_id` returns error at plan time
- [ ] Import by composite ID restores correct state

### `garage_bucket` Data Source

- [ ] Lookup by `id` returns full bucket info
- [ ] Lookup by `global_alias` returns full bucket info
- [ ] Missing both `id` and `global_alias` returns validation error
- [ ] Non-existent bucket returns clear error

### `garage_buckets` Data Source

- [ ] Returns list of all buckets with IDs and aliases
- [ ] Empty cluster returns empty list (not error)

## Implementation Phases

### Phase 1: Bucket Resource

- [ ] Create `internal/provider/resource_bucket.go` with schema definition
- [ ] Implement `Create()`: CreateBucket → AddBucketAlias → UpdateBucket (if quotas/website)
- [ ] Implement `Read()`: GetBucketInfo → map to state, handle 404
- [ ] Implement `Update()`: build UpdateBucketRequestBody, call UpdateBucket
- [ ] Implement `Delete()`: DeleteBucket, handle 400 (not empty)
- [ ] Implement `ImportState()`: ImportStatePassthroughID
- [ ] Add all validators
- [ ] Create `examples/resources/garage_bucket/resource.tf` and `import.sh`

### Phase 2: Bucket Alias Resource

- [ ] Create `internal/provider/resource_bucket_alias.go`
- [ ] Implement Create/Read/Delete for global aliases
- [ ] Implement Create/Read/Delete for local aliases
- [ ] Add cross-attribute validation (alias_type ↔ access_key_id)
- [ ] Implement composite ID parsing for import
- [ ] Create `examples/resources/garage_bucket_alias/resource.tf`

### Phase 3: Bucket Data Sources

- [ ] Create `internal/provider/datasource_bucket.go` with ID/alias lookup
- [ ] Create `internal/provider/datasource_buckets.go` with ListBuckets
- [ ] Add "exactly one of id/global_alias" validation on data source
- [ ] Create examples for both data sources

### Phase 4: Register and Verify

- [ ] Register all bucket resources and data sources in `provider.go`
- [ ] Verify `make build` succeeds
- [ ] Write initial unit tests for bucket schema model mapping

## Test Plan

See [Plan 06 — Testing Strategy](06-testing.md) for complete test definitions.

| Acceptance Criterion | Test ID | Test Type | Location |
|---|---|---|---|
| Bucket create with alias | B1 | Acceptance | `resource_bucket_test.go` |
| Bucket create with website | B2 | Acceptance | `resource_bucket_test.go` |
| Bucket create with quotas | B3 | Acceptance | `resource_bucket_test.go` |
| Bucket update quotas | B4 | Acceptance | `resource_bucket_test.go` |
| Bucket update website toggle | B5 | Acceptance | `resource_bucket_test.go` |
| Bucket import by ID | B6 | Acceptance | `resource_bucket_test.go` |
| Bucket alias change → recreate | B7 | Acceptance | `resource_bucket_test.go` |
| Bucket computed aliases | B8 | Acceptance | `resource_bucket_test.go` |
| Bucket usage metrics readable | B9 | Acceptance | `resource_bucket_test.go` |
| Bucket disappears externally | B10 | Acceptance | `resource_bucket_test.go` |
| Bucket delete non-empty → error | B11 | Acceptance | `resource_bucket_test.go` |
| Bucket invalid alias uppercase | B12 | Acceptance | `resource_bucket_test.go` |
| Bucket invalid alias too long | B13 | Acceptance | `resource_bucket_test.go` |
| Bucket invalid alias empty | B14 | Acceptance | `resource_bucket_test.go` |
| Bucket invalid quota negative | B15 | Acceptance | `resource_bucket_test.go` |
| Alias create global | A1 | Acceptance | `resource_bucket_alias_test.go` |
| Alias create local | A2 | Acceptance | `resource_bucket_alias_test.go` |
| Alias import composite ID | A3 | Acceptance | `resource_bucket_alias_test.go` |
| Alias disappears externally | A4 | Acceptance | `resource_bucket_alias_test.go` |
| Alias local missing key → error | A5 | Acceptance | `resource_bucket_alias_test.go` |
| Alias global with key → error | A6 | Acceptance | `resource_bucket_alias_test.go` |
| Alias duplicate different bucket | A7 | Acceptance | `resource_bucket_alias_test.go` |
| Alias duplicate same bucket (idempotent) | A8 | Acceptance | `resource_bucket_alias_test.go` |
| Data source by ID | D1 | Acceptance | `datasource_bucket_test.go` |
| Data source by alias | D2 | Acceptance | `datasource_bucket_test.go` |
| Data source not found | D3 | Acceptance | `datasource_bucket_test.go` |
| Data source missing both params | D4 | Acceptance | `datasource_bucket_test.go` |
| List buckets | D5 | Acceptance | `datasource_buckets_test.go` |
| List buckets empty | D6 | Acceptance | `datasource_buckets_test.go` |

## Resolved Questions

1. **Bucket name validation regex** — Use S3-compatible rules as a reasonable default: `^[a-z0-9][a-z0-9.-]{0,62}$` (1–63 chars, lowercase alphanumeric + hyphens + dots). If Garage is more permissive, users won't hit issues. If stricter, Garage will reject first.
2. **Update atomicity** — If `UpdateBucket` fails after CreateBucket, the provider stores the bucket ID in state so the next `terraform apply` retries the update. Read always fetches current state from Garage.
3. **Alias collision** — `garage_bucket_alias` Create is **idempotent for the same bucket**: if the alias already exists on the target bucket (e.g., because `garage_bucket` set it as `global_alias`), Create succeeds silently. If the alias exists on a **different** bucket, Create returns a clear error. This means users CAN have overlapping alias references without crashes.
