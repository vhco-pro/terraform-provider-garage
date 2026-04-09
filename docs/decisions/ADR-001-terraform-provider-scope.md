---
description: "Build a Terraform provider as a separate Go module within the garage-operator repo, sharing the OpenAPI-generated client approach"
status: accepted
date: 2025-07-24
author: "garage-operator team"
---

# ADR-001: Terraform Provider Scope and Module Strategy

## Status

Accepted

## Context

The Garage ecosystem lacks a production-quality Terraform provider. The only existing provider ([jkossis/terraform-provider-garage](https://github.com/jkossis/terraform-provider-garage)) covers 27% of the Garage Admin API v2, has a hand-rolled HTTP client with no timeouts or retries, contains a silent no-op update bug on key resources, and has been unmaintained for 5 months with CI disabled. Despite these issues, it has 112K+ downloads on the Terraform Registry, proving market demand.

We already plan to generate a Go client from the Garage Admin API v2 OpenAPI spec (ADR-002). A Terraform provider can reuse the same generation approach. The question is: where does the provider live and how does it relate to the operator?

See: [Terraform Provider Analysis](../research/terraform-provider-analysis.md)

## Decision

We will build a Terraform provider for Garage as a **separate Go module** within the `garage-operator` monorepo, located at `terraform-provider-garage/`. The provider will:

1. **Be a standalone Go module** — `terraform-provider-garage/go.mod` with its own dependency tree
2. **Generate its own API client** — Using the same vendored OpenAPI spec (`internal/garage/openapi/spec.json`) and the same oapi-codegen configuration from ADR-002, but generating into its own `internal/garage/` package
3. **Use terraform-plugin-framework** — The modern HashiCorp provider framework (not the deprecated SDKv2)
4. **Target full API coverage** — All meaningful Garage resources as Terraform resources and data sources
5. **Be export-ready** — The `terraform-provider-garage/` directory can be moved to its own repository at any time with zero changes

### Module Boundaries

```
garage-operator/                    # Root repo
├── go.mod                          # Operator Go module
├── internal/garage/                # Operator's generated client
├── terraform-provider-garage/      # TF provider directory
│   ├── go.mod                      # Separate Go module
│   ├── internal/
│   │   ├── garage/                 # Provider's own generated client
│   │   └── provider/              # TF provider resources/data sources
│   └── main.go
└── internal/garage/openapi/
    └── spec.json                   # Shared OpenAPI spec (read-only input)
```

The only shared artifact is the vendored OpenAPI spec file, which is a **read-only input** to code generation in both modules. There are no Go import dependencies between the modules.

### Resource and Data Source Scope

**Resources (6):**

| Resource | Garage API Operations | Priority |
|---|---|---|
| `garage_bucket` | CreateBucket, GetBucketInfo, UpdateBucket, DeleteBucket | High |
| `garage_key` | CreateKey, ImportKey, GetKeyInfo, UpdateKey, DeleteKey | High |
| `garage_bucket_permission` | AllowBucketKey, DenyBucketKey (via GetBucketInfo) | High |
| `garage_bucket_alias` | AddBucketAlias, RemoveBucketAlias | High |
| `garage_layout_node` | UpdateClusterLayout + ApplyClusterLayout | High |
| `garage_admin_token` | CreateAdminToken, GetAdminTokenInfo, UpdateAdminToken, DeleteAdminToken | Medium |

**Data Sources (10):**

| Data Source | Garage API Operation | Priority |
|---|---|---|
| `garage_bucket` | GetBucketInfo | High |
| `garage_buckets` | ListBuckets | High |
| `garage_key` | GetKeyInfo | High |
| `garage_keys` | ListKeys | High |
| `garage_cluster_health` | GetClusterHealth | Medium |
| `garage_cluster_status` | GetClusterStatus | Medium |
| `garage_cluster_layout` | GetClusterLayout | Medium |
| `garage_admin_token` | GetAdminTokenInfo / GetCurrentAdminTokenInfo | Medium |
| `garage_admin_tokens` | ListAdminTokens | Medium |
| `garage_node_info` | GetNodeInfo | Medium |

### Layout Resource Design

The layout resource deserves special attention. Garage's layout API is two-phase: stage changes, then apply. The Terraform model maps to this as follows:

- `garage_layout_node` represents a single node's role in the layout (zone, capacity, tags)
- On Create/Update: the provider stages the node change via `UpdateClusterLayout`, then applies via `ApplyClusterLayout`
- On Delete: the provider removes the node from the staged layout and applies
- The `ApplyClusterLayout` call requires a layout version number — the provider must read current layout, increment, and apply atomically

This is the single biggest feature gap in the existing provider and the primary differentiator for ours.

## Alternatives Considered

| Alternative | Pros | Cons |
|---|---|---|
| **Fork existing provider** | Faster start, existing tests | Poor foundation (hand-rolled client, no-op bugs), inherit tech debt, GPT-generated code of questionable quality |
| **Shared Go module for API client** | Single source of truth for client code | Creates import dependency between operator and provider, complicates export to separate repo, version coupling |
| **Single Go module (provider inside operator)** | Simpler build | Cannot publish to Terraform Registry as standalone binary, violates Terraform provider conventions, cannot export cleanly |
| **Separate repository from day one** | Clean separation | Harder to coordinate development, spec changes require cross-repo PRs, premature split |

## Consequences

### Positive

- The provider can be exported to its own repo at any time by moving the directory — zero code changes needed
- No Go import coupling between operator and provider — they evolve independently
- Shared OpenAPI spec ensures API client consistency without code coupling
- The provider can be published to the Terraform Registry independently of operator releases
- Both projects benefit from a single source-of-truth OpenAPI spec

### Negative

- Duplicated code generation configuration (two oapi-codegen configs, one per module)
- Duplicated generated client code (one per module)
- Must keep both client generation configs in sync when updating the vendored OpenAPI spec
- Slightly larger repo with two Go modules

### Risks

- Layout resource complexity (two-phase apply, version tracking) may require iteration
- Terraform Registry namespace `garage` may already be taken by jkossis — we'd need a different namespace or coordinate a transfer
