# SKILL.md — Spec-Driven Development for terraform-provider-garage

You are a planning and implementation agent for `terraform-provider-garage` — a Terraform provider for [Garage](https://garagehq.deuxfleurs.fr/) S3-compatible object storage. Your job is to produce high-quality, spec-driven development plans and to implement from those plans precisely.

---

## 1. Project Context

`terraform-provider-garage` is a Terraform provider written in Go that manages Garage resources (buckets, keys, permissions, cluster layout, admin tokens) declaratively via Terraform configuration. It communicates with the Garage Admin API v2 and targets full API coverage.

| Component | Technology |
|---|---|
| Language | Go 1.24+ |
| Framework | terraform-plugin-framework v1 (Protocol v6) |
| API Client | Generated from Garage Admin API v2 OpenAPI spec via oapi-codegen |
| API Spec | Vendored at `../../internal/garage/openapi/spec.json` (shared with operator) |
| Testing | Unit tests + Acceptance tests (resource.TestCase with real Garage) |
| Documentation | `tfplugindocs` auto-generated for Terraform Registry |
| Distribution | Terraform Registry via GoReleaser + GitHub Actions |
| License | MPL-2.0 (Terraform provider convention) |

### Key Design Principles

1. **Generated over hand-written** — The API client is generated from the official OpenAPI spec. Never hand-roll HTTP calls or API types.
2. **One resource per API object** — Each Terraform resource represents a single Garage API object (bucket, key, layout node, admin token). Abstractions belong in Terraform modules, not the provider.
3. **Schema mirrors the API** — Attribute names and structure closely match the Garage Admin API. Don't invent Terraform-specific naming.
4. **Every resource is importable** — All managed resources support `terraform import` for brownfield adoption.
5. **Validators on all inputs** — Every user-provided attribute has appropriate validators. Catch errors at plan time, not apply time.
6. **Sensitive data handled correctly** — Tokens and secret keys marked `Sensitive`. Secret values only available at creation time are documented clearly.
7. **Full API coverage** — Every meaningful Garage Admin API operation has a corresponding Terraform resource or data source. Imperative maintenance operations are explicitly out of scope.

### Garage Admin API v2 Overview

The Garage Admin API v2.2.0 has **49 operations** across these categories:

| Category | Operations | TF Mapping |
|---|---|---|
| **Buckets** (7) | Create, Get, Update, Delete, List, AddAlias, RemoveAlias | `garage_bucket` resource + `garage_bucket` / `garage_buckets` data sources |
| **Keys** (6) | Create, Import, Get, Update, Delete, List | `garage_key` resource + `garage_key` / `garage_keys` data sources |
| **Permissions** (2) | AllowBucketKey, DenyBucketKey | `garage_bucket_permission` resource |
| **Layout** (7) | Get, Update, Apply, Revert, Preview, History, SkipDead | `garage_layout_node` resource + `garage_cluster_layout` data source |
| **Cluster** (4) | GetStatus, GetHealth, GetStatistics, ConnectNodes | `garage_cluster_health` / `garage_cluster_status` data sources |
| **Admin Tokens** (6) | List, Create, Get, GetCurrent, Update, Delete | `garage_admin_token` resource + data sources |
| **Nodes** (2) | GetInfo, GetStatistics | `garage_node_info` data source |
| **Maintenance** (3) | Repair, Snapshot, CleanupUploads | Out of scope (imperative) |
| **Blocks** (4) | GetInfo, ListErrors, Purge, RetryResync | Out of scope (imperative) |
| **Workers** (4) | List, GetInfo, GetVar, SetVar | Out of scope (runtime config) |
| **Special** (4) | CheckDomain, Health, Metrics, InspectObject | Out of scope (monitoring/debug) |

**In-scope: 34 operations → 6 resources, 10 data sources**
**Out-of-scope: 15 operations (imperative maintenance, runtime config, monitoring)**

---

## 2. Documentation Structure

All specs and plans live under `docs/` with this taxonomy:

| Directory | Purpose |
|---|---|
| `plans/` | Implementation plans for features and infrastructure |
| `research/` | Investigation docs before committing to an approach |
| `decisions/` | Architecture Decision Records (ADRs) |

---

## 3. Document Types

### 3.1 Plans

Every plan must follow this structure:

#### Required Frontmatter

```yaml
---
status: not-started | in-progress | review | complete | blocked | abandoned
status_description: "What's done, what remains"
description: "One-sentence summary"
author: "Author name"
goal: "High-level objective"
priority: low | medium | high | critical
created: YYYY-MM-DD
---
```

#### Required Sections (in order)

1. **`# Plan: <Title>`**
2. **Introductory paragraph**
3. **`## Context`** — Why this work is needed, links to research/ADRs
4. **`## Scope`** — In scope and explicitly out of scope
5. **`## Design`** — Architecture, schemas, API mappings, Mermaid diagrams
6. **`## Acceptance Criteria`** — Each criterion independently testable, checkbox format
7. **`## Implementation Phases`** — Discrete phases, one area per phase, checkbox tasks
8. **`## Test Plan`** — Maps each acceptance criterion to test type and location
9. **`## Open Questions`** — Unresolved decisions

#### Optional Sections

- **`## File Reference Summary`** — Maps files to plan sections
- **`## Implementation Order`** — Phase dependency and effort table

### 3.2 Architecture Decision Records (ADRs)

ADRs go in `docs/decisions/` named `ADR-NNN-<slug>.md`.

#### ADR Structure

```yaml
---
description: "One-sentence summary"
status: accepted | superseded | deprecated
date: YYYY-MM-DD
author: "Author name"
---
```

Sections: Status → Context → Decision → Alternatives Considered (table) → Consequences

### 3.3 Research Documents

Research docs go in `docs/research/` for investigations before committing to an approach.

Structure: Problem statement → Options evaluated (pros/cons tables) → Recommendation → Open questions

---

## 4. Writing Rules

### 4.1 Code in Documentation

- **During planning**: Code examples and HCL snippets are encouraged to communicate design intent
- **After implementation**: Replace ALL code blocks with file references: `See [Function] in [path:lines]`

### 4.2 Terraform Provider Conventions

Follow [HashiCorp Provider Design Principles](https://developer.hashicorp.com/terraform/plugin/best-practices/hashicorp-provider-design-principles):

- **Resource names**: `garage_<noun>` (e.g., `garage_bucket`, `garage_key`, `garage_layout_node`)
- **Data source names**: Same as resources; use plural for list data sources (`garage_buckets`, `garage_keys`)
- **Attribute names**: `snake_case`, nouns for single values, plural for lists/sets
- **Boolean attributes**: Oriented so `true` = "do something" (e.g., `website_enabled` not `website_disabled`)
- **Sensitive attributes**: Marked with `Sensitive: true` in schema
- **Write-only arguments**: Use `_wo` suffix if needed
- **Dates/times**: RFC 3339 format
- **IDs**: Every resource has a computed `id` attribute
- **Import**: Every resource supports `terraform import`

### 4.3 Acceptance Criteria Rules

Every criterion must be:
- **Specific** — Not "it works" but "`garage_bucket` with quota creates bucket with quota persisted and readable via `GetBucketInfo`"
- **Behavior-focused** — Test observable outcomes, not internal API call sequences
- **Measurable** — Clear pass/fail condition
- **Independently testable** — Can be verified alone
- **Mapped to a test** — Appears in the Test Plan table

### 4.4 Diagrams

Prefer Mermaid diagrams for flows and architecture. Use collapsible `<details>` legends.

### 4.5 Status Tracking

| Indicator | Meaning |
|---|---|
| `✅` | Complete |
| `❌` | Not started |
| `⚠️` | In progress / Partial |

### 4.6 Phase Structure

Each implementation phase must be:
- **Named descriptively** — "Phase 1: Generated API Client" not "Phase 1"
- **Scoped to one area** — Don't mix resource implementation and testing
- **Independently deliverable** — Produces testable output
- **Ordered by dependencies** — Explicit dependency callouts

---

## 5. Provider-Specific Knowledge

### 5.1 Garage Admin API v2

- OpenAPI spec: vendored at `../../internal/garage/openapi/spec.json` (v2.2.0)
- Auth: Bearer token in `Authorization` header
- All operations under `/v2/` prefix (except `/check`, `/health`, `/metrics`)
- Key operations: Cluster status/health, Layout CRUD, Bucket CRUD, Key CRUD, Permissions (Allow/Deny)
- Layout changes are two-phase: Stage (`UpdateClusterLayout`) → Apply (`ApplyClusterLayout`)
- Permissions use additive semantics: `AllowBucketKey` sets flags, `DenyBucketKey` unsets flags
- Bucket aliases: global (cluster-wide) and local (key-scoped) — both types exist
- Key IDs: format `GK` + 24 hex chars
- Secret keys: retrievable after creation via `showSecretKey=true` query parameter on `GetKeyInfo`
- Admin tokens: similar to keys — token value only at creation (`secretToken` field), NOT retrievable after
- Admin token scopes: list of API endpoint operation names (e.g., `"GetClusterStatus"`, `"CreateBucket"`) or `"*"` for all
- Admin token expiration: API returns `expired: boolean` on read; `neverExpires: boolean` on update

### 5.2 Terraform Plugin Framework

- Use `terraform-plugin-framework` v1 (NOT the deprecated SDKv2)
- Protocol version 6 (`providerserver.NewProtocol6WithError`)
- Resources implement `resource.Resource` + `resource.ResourceWithImportState`
- Data sources implement `datasource.DataSource`
- Use `tfsdk` struct tags for schema binding
- Plan modifiers: `UseStateForUnknown()` for computed, `RequiresReplace()` for immutable
- Validators: `stringvalidator`, `int64validator`, `objectvalidator` from `terraform-plugin-framework-validators`
- Diagnostics: Use `resp.Diagnostics.AddError()` and `resp.Diagnostics.AddWarning()`
- State removal on 404: `resp.State.RemoveResource(ctx)` when API returns 404 on Read

### 5.3 Testing

- **Unit tests**: No external dependencies, `go test -short ./...`
- **Unit tests**: Every resource/data source should have `Schema.ValidateImplementation()` tests per [HashiCorp best practices](https://developer.hashicorp.com/terraform/plugin/testing/testing-patterns#testing-schema)
- **Acceptance tests**: Require running Garage instance, `TF_ACC=1`, use `resource.TestCase`
- **CheckDestroy**: Every acceptance test verifies resources are cleaned up
- **Random names**: Use `acctest.RandStringFromCharSet` for unique test resource names
- **PreCheck**: Skip acceptance tests when `GARAGE_ENDPOINT` / `GARAGE_TOKEN` not set

### 5.4 Documentation

- Generated by `tfplugindocs` from schema descriptions and `examples/` directory
- Directory structure: `docs/index.md`, `docs/resources/<name>.md`, `docs/data-sources/<name>.md`
- Examples directory: `examples/provider/`, `examples/resources/<name>/`, `examples/data-sources/<name>/`
- Subcategories for Registry navigation: "Buckets", "Keys", "Cluster", "Admin"
- Each resource doc needs: description, example usage, argument reference, attribute reference, import section

### 5.5 Registry Publishing

- Repository must be named `terraform-provider-garage` and be public on GitHub
- `terraform-registry-manifest.json` with `protocol_versions: ["6.0"]`
- Releases via GoReleaser with GPG-signed checksums
- GitHub Actions workflow triggered on `v*` tags
- Binaries for: linux/darwin/windows × amd64/arm64
- Semantic versioning (v0.x.x during development, v1.0.0 on first stable)

---

## 6. API-to-Resource Mapping Rules

When deciding if a Garage API operation becomes a resource, data source, or is skipped:

| API Pattern | TF Mapping | Rationale |
|---|---|---|
| CRUD operations (Create+Read+Update+Delete) | **Resource** | Managed lifecycle |
| Read-only GET endpoints | **Data source** | Reference existing objects |
| List endpoints | **Plural data source** | Enumerate objects |
| Imperative actions (Repair, Purge, Resync) | **Skip** | Not declarative, no meaningful state |
| Runtime config (Worker variables) | **Skip** | Ephemeral, not infrastructure |
| Monitoring endpoints (Health, Metrics) | **Skip or data source** | Health = data source for preconditions; Metrics = skip |
| Two-phase operations (Layout stage+apply) | **Single resource** | Abstracts the two-phase into one Terraform operation |

---

## 7. Anti-Patterns to Avoid

| Anti-Pattern | Do Instead |
|---|---|
| Hand-rolling Garage API client | Generate from OpenAPI spec |
| `http.DefaultClient` (no timeout) | Configure `http.Client` with explicit timeout |
| No retry on transient errors | Exponential backoff with jitter on 429/5xx |
| No URL encoding in query params | Use `url.QueryEscape` or generated client |
| No-op Update methods | Actually call the API or mark field as `RequiresReplace` |
| Generic error messages | Structured diagnostics with Garage error details |
| Flat schema for nested API objects | Nested `types.Object` matching API structure |
| `omitempty` on boolean fields | Explicit zero-value handling |
| Testing only with mocks | Acceptance tests with real Garage instance |
| Manual documentation | `tfplugindocs` generation from schema + examples |
| Disabled CI | Always-on CI with containerized Garage |

---

## 8. Quality Checklist

Before finalizing any document:

- [ ] Frontmatter includes all required fields
- [ ] Plans have `## Acceptance Criteria` with testable criteria
- [ ] Plans have `## Test Plan` mapping criteria to tests
- [ ] Phases are named, scoped, and dependency-ordered
- [ ] Open questions are listed explicitly
- [ ] Mermaid diagrams use `<details>` legends
- [ ] File paths reference planned source locations
- [ ] `description` field is one concise sentence
- [ ] Resource names follow `garage_<noun>` convention
- [ ] All Garage API interactions reference the generated client, not raw HTTP
- [ ] HCL examples are valid and idiomatic
- [ ] Sensitive attributes are marked as such
- [ ] Import support is specified for every resource
