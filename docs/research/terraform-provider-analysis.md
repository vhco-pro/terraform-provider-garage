# Research: Existing Terraform Provider for Garage

## Subject

[jkossis/terraform-provider-garage](https://github.com/jkossis/terraform-provider-garage) — the only Terraform provider for Garage on the HashiCorp Registry.

## Summary

The provider is functional for basic bucket/key/permission management but covers only 27% of the Garage Admin API v2. It has significant code quality issues (hand-rolled client, no timeouts, no retries, a silent no-op update bug), disabled CI, and a single AI-assisted contributor with no updates in 5 months. Despite this, it has **112K+ downloads** on the Terraform Registry, demonstrating clear market demand. There is a strong opportunity to build a superior provider.

---

## Provider Profile

| Attribute | Value |
|---|---|
| **Version** | v1.0.4 |
| **Published** | November 2025 |
| **Last Commit** | 5 months ago |
| **Stars** | 13 |
| **Forks** | 2 |
| **Contributors** | 1 (jkossis) |
| **Registry Downloads** | 112,663 total / 9,383 weekly |
| **License** | MPL-2.0 |
| **Framework** | terraform-plugin-framework v1.16.1 (modern, correct choice) |
| **Origin** | AI-generated — commit messages include "claude made this for me lol" |
| **Scaffolding** | hashicorp/terraform-provider-scaffolding-framework |
| **Go Version** | 1.24 |
| **Garage Version** | >= 0.9.0 (Admin API v2) |

---

## Feature Coverage

### Resources (3 of ~8 possible)

| Resource | CRUD | Import | Notes |
|---|---|---|---|
| `garage_bucket` | ✅ Full | ✅ PassthroughID | Website hosting, quotas |
| `garage_key` | C✅ R✅ U⚠️ D✅ | ✅ PassthroughID | **Update is a no-op** — name changes silently dropped |
| `garage_bucket_permission` | ✅ Full | ✅ Custom composite ID | Correctly handles Allow/Deny semantics |

### Data Sources (1 of ~6 possible)

| Data Source | Lookup By |
|---|---|
| `garage_bucket` | `id` or `global_alias` |

### API Coverage: 13 of 49 operations (27%)

| Category | Operations | Covered |
|---|---|---|
| **Buckets** | CreateBucket, GetBucketInfo, UpdateBucket, DeleteBucket, ListBuckets, AddBucketAlias, RemoveBucketAlias | 4 as resources, 3 in client only (unused) |
| **Keys** | CreateKey, GetKeyInfo, ImportKey, DeleteKey, UpdateKey, ListKeys | 4 used, UpdateKey explicitly skipped, ListKeys missing |
| **Permissions** | AllowBucketKey, DenyBucketKey | ✅ Both |
| **Cluster** | GetClusterStatus, GetClusterHealth, GetClusterStatistics, ConnectClusterNodes | ❌ None |
| **Layout** | GetClusterLayout, UpdateClusterLayout, ApplyClusterLayout, RevertClusterLayout, PreviewChanges, GetHistory, SkipDeadNodes | ❌ None |
| **Admin Tokens** | List, Create, Get, GetCurrent, Update, Delete | ❌ None |
| **Nodes** | GetNodeInfo, GetNodeStatistics | ❌ None |
| **Maintenance** | LaunchRepair, CreateSnapshot, CleanupUploads | ❌ None |
| **Blocks** | GetBlockInfo, ListBlockErrors, PurgeBlocks, RetryBlockResync | ❌ None |
| **Workers** | ListWorkers, GetWorkerInfo, GetWorkerVariable, SetWorkerVariable | ❌ None |
| **Special** | CheckDomain, Health, Metrics, InspectObject | ❌ None |

---

## Code Quality Assessment

### What's Good

1. **Framework choice** — Uses terraform-plugin-framework v1.16.1 (modern, not the deprecated SDKv2)
2. **Permission handling** — `BucketPermissionResource.Update()` correctly diffs old/new state and calls Allow/Deny only for changed permissions
3. **Plan modifiers** — Proper use of `RequiresReplace()` for immutable fields, `UseStateForUnknown()` for computed values
4. **Sensitive fields** — `secret_access_key` and `token` properly marked sensitive
5. **Import support** — All 3 resources support `terraform import`, permission resource uses composite ID
6. **Acceptance tests** — 23 acceptance tests covering all resources and the data source

### What's Wrong

#### 1. Hand-Rolled API Client

The client in `internal/client/client.go` is entirely hand-written with raw `net/http` calls. Garage provides an OpenAPI 3.1 spec — this should be generated.

```
// Example from the client — manual JSON marshaling, manual URL construction
path += "id=" + *req.ID  // No url.QueryEscape!
```

#### 2. No HTTP Timeouts

Uses `http.DefaultClient` which has **zero timeout**. A hung Garage server will hang Terraform indefinitely.

#### 3. No Retry Logic

Any transient 500 or network error immediately fails the operation. No exponential backoff, no retry on transient failures.

#### 4. Key Update No-Op Bug

The `Update` method on `KeyResource` is literally a no-op:

```go
func (r *KeyResource) Update(...) {
    // Note: UpdateKey is available in the API but we're not implementing it
    tflog.Trace(ctx, "Updated access key resource (no-op)")
    resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
```

Changing a key's `name` in Terraform config shows "apply complete" but **nothing changes in Garage**. This is a data integrity bug — Terraform state diverges from reality.

#### 5. No URL Encoding

Query parameters are concatenated as raw strings: `path += "id=" + *req.ID`. Bucket IDs or aliases containing `&`, `=`, or spaces will break silently.

#### 6. No Input Validation

Zero `Validators` on any schema attribute. No validation of endpoint URL format, token format, bucket name format, or quota values.

#### 7. Client Methods Implemented but Unused

`ListBuckets`, `AddBucketAlias`, and `RemoveBucketAlias` are implemented in the client but have no corresponding Terraform resource or data source.

#### 8. No Error Typing

All errors are generic `fmt.Errorf("API request failed with status %d: %s")`. No structured error types, no differentiation between 4xx and 5xx, no Terraform-friendly diagnostic messages.

#### 9. CI Disabled

The `.github/workflows/` directory exists but the commit message says **"don't run tests for now"**. Tests require a running Garage instance with no automated setup.

#### 10. Website State Asymmetry

`WebsiteAccess` is stored as a simple bool but the API returns both a boolean and a `WebsiteConfig` object. The Read/Create mapping is asymmetric and could cause state drift.

---

## Testing Assessment

| Category | Count | Quality |
|---|---|---|
| Acceptance tests (resources) | 17 | Good coverage of CRUD, edge cases |
| Acceptance tests (data sources) | 6 | Good coverage including lookup variants |
| Client unit tests | ~8 | Basic httptest mocks, decent |
| CI/CD | Disabled | Tests exist but don't run automatically |

The acceptance tests are the strongest part of the codebase. They properly use `resource.TestCase` with `ProtoV6ProviderFactories` and cover important scenarios like key replacement on change, permission toggling, and website configuration.

---

## Missing Resources — Priority Analysis

| Resource | Priority | Rationale |
|---|---|---|
| `garage_layout` | **Critical** | Required for any serious Garage deployment — node zone/capacity assignment |
| `garage_key` update | **High** | Current no-op bug means name changes are silently lost |
| `garage_admin_token` | **Medium** | Needed for multi-tenant and automation scenarios |
| `garage_key` data source | **Medium** | No way to reference existing keys without managing them |
| `garage_bucket` list data source | **Medium** | No way to enumerate buckets |
| `garage_cluster_health` data source | **Medium** | Health checks and preconditions in Terraform |
| `garage_cluster_status` data source | **Low** | Node discovery and cluster info |

---

## Market Opportunity

| Signal | Assessment |
|---|---|
| **Demand** | 112K downloads, 9K weekly — significant for a niche provider |
| **Competition** | This is the **only** Garage Terraform provider |
| **Maintenance** | Single contributor, AI-generated, 5 months stale, CI disabled |
| **Feature gaps** | 73% of API uncovered — layout, cluster, admin tokens all missing |
| **Code quality** | Multiple bugs and security issues in hand-rolled client |
| **Open PRs** | 4 PRs unreviewed — suggests abandoned maintenance |

The combination of proven demand (112K downloads) and clear deficiencies (bugs, missing features, abandoned maintenance) makes this a strong opportunity. A well-built provider with full API coverage and proper engineering practices would be immediately competitive.

---

## Recommendation

**Build a new provider** from scratch inside the `garage-operator` repo as `terraform-provider-garage/`. Key advantages over the existing provider:

1. **Generated API client** — Share the oapi-codegen approach from ADR-002, generating from the same vendored OpenAPI spec
2. **Full API coverage** — Target all meaningful Garage resources, not just buckets and keys
3. **Layout management** — The single biggest missing feature, critical for real deployments
4. **Proper HTTP client** — Configured timeouts, retry with backoff, URL encoding
5. **Validators and diagnostics** — Input validation, structured errors, actionable diagnostics
6. **Working CI** — Automated testing with containerized Garage instance
7. **Registry publishing** — Automated release pipeline via GoReleaser

The provider should be a **separate Go module** within the repo (`terraform-provider-garage/go.mod`) so it can be cleanly exported to its own repository later. It should use the same OpenAPI spec and client generation approach as the operator but maintain its own generated client to avoid cross-module dependencies.
