---
status: done
status_description: "Fully implemented. 22 example files (6 resources + 10 data sources + import scripts), templates/index.md.tmpl, tfplugindocs generate/validate pass, docs/resources/ (6 files) + docs/data-sources/ (10 files) generated, .github/workflows/release.yml, docs + docs-validate Makefile targets."
description: "Documentation generation, Terraform Registry structure, and release pipeline"
author: "garage-operator team"
goal: "Publish provider to Terraform Registry with auto-generated documentation and automated releases"
priority: medium
created: 2026-04-06
---

# Plan: Documentation & Registry Publishing

This plan covers three areas: auto-generated provider documentation, Terraform Registry publishing, and the GoReleaser-based release pipeline.

## Context

The [existing provider](../research/terraform-provider-analysis.md) is published on the Registry with 112K+ downloads but has minimal documentation and a broken release pipeline (`ci: false`). We aim for Registry-quality docs generated from schema, working examples, and a fully automated release process.

Depends on: All resource/data source plans, [Plan 06 — Testing](06-testing.md)

## 1. Documentation

### Tool: `tfplugindocs`

[`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs) generates Markdown documentation from:
- Provider/resource/data source schemas (descriptions, types, required/optional/computed)
- Template files in `templates/`
- Example HCL in `examples/`

### Directory Structure

```
terraform-provider-garage/
├── docs/                          # Generated — DO NOT EDIT
│   ├── index.md                   # Provider overview (from templates/)
│   ├── resources/
│   │   ├── bucket.md
│   │   ├── bucket_alias.md
│   │   ├── key.md
│   │   ├── bucket_permission.md
│   │   ├── layout_node.md
│   │   └── admin_token.md
│   └── data-sources/
│       ├── bucket.md
│       ├── buckets.md
│       ├── key.md
│       ├── keys.md
│       ├── cluster_health.md
│       ├── cluster_status.md
│       ├── cluster_layout.md
│       ├── node_info.md
│       ├── admin_token.md
│       └── admin_tokens.md
├── templates/                     # Editable templates
│   └── index.md.tmpl              # Provider index page template
├── examples/                      # HCL examples (referenced by docs)
│   ├── provider/
│   │   └── provider.tf            # Provider configuration
│   ├── resources/
│   │   ├── garage_bucket/
│   │   │   ├── resource.tf
│   │   │   └── import.sh
│   │   ├── garage_bucket_alias/
│   │   │   ├── resource.tf
│   │   │   └── import.sh
│   │   ├── garage_key/
│   │   │   ├── resource.tf
│   │   │   └── import.sh
│   │   ├── garage_bucket_permission/
│   │   │   ├── resource.tf
│   │   │   └── import.sh
│   │   ├── garage_layout_node/
│   │   │   ├── resource.tf
│   │   │   └── import.sh
│   │   └── garage_admin_token/
│   │       ├── resource.tf
│   │       └── import.sh
│   └── data-sources/
│       ├── garage_bucket/
│       │   └── data-source.tf
│       ├── garage_buckets/
│       │   └── data-source.tf
│       ├── garage_key/
│       │   └── data-source.tf
│       ├── garage_keys/
│       │   └── data-source.tf
│       ├── garage_cluster_health/
│       │   └── data-source.tf
│       ├── garage_cluster_status/
│       │   └── data-source.tf
│       ├── garage_cluster_layout/
│       │   └── data-source.tf
│       ├── garage_node_info/
│       │   └── data-source.tf
│       ├── garage_admin_token/
│       │   └── data-source.tf
│       └── garage_admin_tokens/
│           └── data-source.tf
```

### Schema Descriptions

Every attribute in every schema **must** have a `Description` field. `tfplugindocs` uses these to generate attribute tables. Write descriptions in the schema definition, not in separate docs:

```go
schema.StringAttribute{
    Description: "Human-friendly name for the bucket. Used as the global alias.",
    Required:    true,
}
```

Description guidelines:
- Start with noun phrase, not "The" or "A"
- Include valid values/constraints where relevant
- Note sensitivity: "Sensitive. Only available at creation time."
- Note computed: "Set by Garage."

### Provider Index Template

`templates/index.md.tmpl`:

```markdown
---
page_title: "garage Provider"
subcategory: ""
description: |-
  Manage Garage S3-compatible object storage via the Admin API.
---

# garage Provider

The garage provider allows you to manage [Garage](https://garagehq.deuxfleurs.fr/) 
S3-compatible object storage resources via the Admin API (v2).

## Authentication

The provider requires an admin API token. Configure it via the provider block 
or the `GARAGE_TOKEN` environment variable.

{{ .SchemaMarkdown | trimspace }}

## Example Usage

{{ tffile "examples/provider/provider.tf" }}
```

### Example Files

#### Provider Example

`examples/provider/provider.tf`:

```hcl
provider "garage" {
  endpoint = "http://garage.example.com:3903"
  token    = var.garage_admin_token
}
```

#### Resource Example (bucket)

`examples/resources/garage_bucket/resource.tf`:

```hcl
resource "garage_bucket" "website" {
  global_alias = "my-website"

  website = {
    enabled        = true
    index_document = "index.html"
    error_document = "error.html"
  }

  quotas = {
    max_size    = 5368709120  # 5 GiB
    max_objects = 100000
  }
}
```

#### Import Example

`examples/resources/garage_bucket/import.sh`:

```bash
terraform import garage_bucket.website <bucket-id>
```

### Subcategories

Subcategories are set in the schema code on each resource/data source via the `Description` field of the schema response's metadata. `tfplugindocs` picks up the `subcategory` from the generated doc frontmatter. Set it in the schema:

```go
func (r *BucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_bucket"
}
```

The subcategory is set in the template or via `tfplugindocs` `--subcategory` flag per resource.

| Subcategory | Resources | Data Sources |
|---|---|---|
| Buckets | `garage_bucket`, `garage_bucket_alias` | `garage_bucket`, `garage_buckets` |
| Keys | `garage_key`, `garage_bucket_permission` | `garage_key`, `garage_keys` |
| Cluster | `garage_layout_node` | `garage_cluster_health`, `garage_cluster_status`, `garage_cluster_layout`, `garage_node_info` |
| Admin | `garage_admin_token` | `garage_admin_token`, `garage_admin_tokens` |

### Generating Docs

```bash
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
tfplugindocs generate --provider-name garage
```

Add to Makefile:

```makefile
.PHONY: docs
docs:
	tfplugindocs generate --provider-name garage

.PHONY: docs-validate
docs-validate:
	tfplugindocs validate --provider-name garage
```

CI must run `docs-validate` to catch schema/docs drift.

### Guides (Optional)

For common workflows, add guide pages in `templates/guides/`:

| Guide | Content |
|---|---|
| `getting-started.md.tmpl` | Install provider, configure, create first bucket |
| `bucket-permissions.md.tmpl` | Key + bucket + permission pattern |
| `cluster-layout.md.tmpl` | Layout management with two-phase apply |

Guides appear in the Registry sidebar under "Guides".

## 2. Terraform Registry Publishing

### Prerequisites

1. **GitHub repository**: `github.com/vhco-pro/terraform-provider-garage`
2. **GPG signing key**: For release artifact signatures
3. **Registry account**: Sign in at [registry.terraform.io](https://registry.terraform.io) with GitHub
4. **Webhook**: Automatically configured when publishing via Registry UI

### Registry Manifest

`terraform-registry-manifest.json` (root of repo):

```json
{
  "version": 1,
  "metadata": {
    "protocol_versions": ["6.0"]
  }
}
```

### Publishing Steps

1. Push code to public GitHub repo matching `terraform-provider-<NAME>` naming
2. Sign in to [registry.terraform.io](https://registry.terraform.io)
3. Navigate to "Publish" → "Provider"
4. Select repository
5. Registry auto-detects releases via GitHub webhooks
6. Each GitHub release with signed artifacts becomes a Registry version

### Versioning Strategy

| Phase | Version | Stability |
|---|---|---|
| Initial development | `v0.1.0` | Breaking changes allowed |
| Feature complete | `v0.x.y` | Resources stable, minor polish |
| Production ready | `v1.0.0` | Semantic versioning, no breaking changes without major bump |

Follow [HashiCorp versioning guidance](https://developer.hashicorp.com/terraform/plugin/best-practices/versioning):
- `v0.x`: rapid iteration, API may change
- `v1.0`: stable resource schemas, import compatibility guaranteed

## 3. Release Pipeline

### GoReleaser Configuration

`.goreleaser.yml`:

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go generate ./...
    - go test ./...

builds:
  - env:
      - CGO_ENABLED=0
    mod_timestamp: "{{ .CommitTimestamp }}"
    flags:
      - -trimpath
    ldflags:
      - "-s -w -X main.version={{ .Version }} -X main.commit={{ .Commit }}"
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    binary: "{{ .ProjectName }}_v{{ .Version }}"

archives:
  - format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_SHA256SUMS"
  algorithm: sha256

signs:
  - artifacts: checksum
    args:
      - "--batch"
      - "--local-user"
      - "{{ .Env.GPG_FINGERPRINT }}"
      - "--output"
      - "${signature}"
      - "--detach-sign"
      - "${artifact}"

release:
  draft: false
  prerelease: auto

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
```

### GitHub Actions: Release

`.github/workflows/release.yml`:

```yaml
name: Release
on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Import GPG key
        uses: crazy-max/ghaction-import-gpg@v6
        id: import_gpg
        with:
          gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}
          passphrase: ${{ secrets.GPG_PASSPHRASE }}
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GPG_FINGERPRINT: ${{ steps.import_gpg.outputs.fingerprint }}
```

### GPG Key Setup

1. Generate a dedicated GPG key for signing releases (with 2-year expiry):
   ```bash
   gpg --batch --gen-key <<EOF
   Key-Type: RSA
   Key-Length: 4096
   Name-Real: Terraform Provider Garage Release
   Name-Email: release@vhco.pro
   Expire-Date: 2y
   %no-protection
   EOF
   ```
2. Export and add to GitHub secrets:
   - `GPG_PRIVATE_KEY`: `gpg --armor --export-secret-keys <fingerprint>`
   - `GPG_PASSPHRASE`: passphrase (empty if `%no-protection`)
3. Add public key to Terraform Registry (Settings → GPG keys)

### Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/) for GoReleaser changelog filtering:

| Prefix | Purpose | In changelog? |
|---|---|---|
| `feat:` | New feature | Yes |
| `fix:` | Bug fix | Yes |
| `docs:` | Documentation | No (filtered) |
| `test:` | Tests | No (filtered) |
| `ci:` | CI/CD | No (filtered) |
| `chore:` | Maintenance | No |
| `BREAKING CHANGE:` | Breaking change | Yes (major bump) |

A breaking change in `v0.x` is a schema change (rename attribute, change type, remove attribute). Breaking changes require a new minor version (`v0.2.0`) until `v1.0.0`.

### Release Workflow

```
git tag v0.1.0
git push origin v0.1.0
→ GitHub Actions triggers
  → GoReleaser builds 6 binaries (linux/darwin × amd64/arm64, windows × amd64/arm64)
  → Signs SHA256SUMS with GPG
  → Creates GitHub Release with artifacts
→ Registry webhook fires
  → Registry downloads and indexes the release
  → Provider available via `terraform init`
```

## Acceptance Criteria

### Documentation

- [ ] Every schema attribute has a `Description` field
- [ ] `tfplugindocs generate` produces docs for all 6 resources and 10 data sources
- [ ] `tfplugindocs validate` passes with no errors
- [ ] Provider index page includes authentication and example usage
- [ ] Every resource has an example in `examples/resources/`
- [ ] Every data source has an example in `examples/data-sources/`
- [ ] Every resource has an `import.sh` example
- [ ] `docs-validate` runs in CI

### Registry

- [ ] `terraform-registry-manifest.json` exists with protocol version 6.0
- [ ] Repository follows `terraform-provider-<NAME>` naming
- [ ] GPG public key uploaded to Registry
- [ ] Provider page shows all resources and data sources in correct subcategories

### Release Pipeline

- [ ] `.goreleaser.yml` builds for linux/darwin/windows × amd64/arm64
- [ ] Release artifacts are GPG-signed
- [ ] `go test` runs as a pre-release hook
- [ ] GitHub Actions release workflow triggers on `v*` tags
- [ ] Tags follow semver (`v0.1.0`, `v1.0.0`)

## Implementation Phases

### Phase 1: Documentation Infrastructure

- [ ] Install `tfplugindocs`
- [ ] Create `templates/index.md.tmpl`
- [ ] Create all example files (provider, 6 resources, 10 data sources)
- [ ] Add `docs` and `docs-validate` Makefile targets
- [ ] Run `tfplugindocs generate` and verify output

### Phase 2: Release Pipeline

- [ ] Create `.goreleaser.yml`
- [ ] Create `.github/workflows/release.yml`
- [ ] Create `terraform-registry-manifest.json`
- [ ] Generate GPG key and configure GitHub secrets
- [ ] Test with `goreleaser release --snapshot --clean`

### Phase 3: Registry Publishing

- [ ] Push to public GitHub repo
- [ ] Publish on registry.terraform.io
- [ ] Verify provider page, subcategories, and docs
- [ ] Tag `v0.1.0` and verify automated release flow

### Phase 4: CI Documentation Validation

- [ ] Add `docs-validate` step to CI workflow
- [ ] Add `tfplugindocs validate` to PR checks
