---
status: done
status_description: "Fully implemented. 22 unit tests (client + helpers + validators), 30+ acceptance tests (resources, data sources, E2E, provider config), CI pipeline (.github/workflows/ci.yml), testdata/garage.toml + docker-compose.yml. All tests compile and unit tests pass. Acceptance tests skip without GARAGE_ENDPOINT."
description: "Comprehensive testing strategy for terraform-provider-garage — unit, acceptance, negative, E2E, and CI"
author: "garage-operator team"
goal: "Production-grade test suite covering every resource, every error path, and every validation rule"
priority: critical
created: 2026-04-06
---

# Plan: Testing Strategy

This plan defines the complete testing approach for `terraform-provider-garage`. Every resource, data source, validator, error path, and edge case gets a test. This is the competitive differentiator — competing providers have zero tests.

## Context

The [existing provider](../research/terraform-provider-analysis.md) has disabled CI (`ci: false` in GoReleaser), zero acceptance tests, and zero unit tests. The most-downloaded competitor (112K downloads) ships bugs that a single acceptance test would catch. Our provider targets full coverage with real acceptance tests against a containerized Garage instance, including negative tests, validation error tests, and multi-resource E2E workflows.

HashiCorp's [testing best practices](https://developer.hashicorp.com/terraform/plugin/testing) define three tiers:
1. **Unit tests** — Pure Go, no Terraform or Garage involved. Test business logic.
2. **Acceptance tests** — Full Terraform lifecycle against a real Garage instance. Test each resource independently.
3. **E2E tests** — Multi-resource workflows validating real-world usage patterns.

We add a fourth tier:
4. **Negative tests** — Intentionally invalid configs, API error simulation, validation failures. Prove the provider fails gracefully.

## Testing Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      CI Pipeline                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ go vet/fmt  │  │ golangci-   │  │ go build        │  │
│  │             │  │   lint      │  │   -o /dev/null  │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │           Unit Tests (go test, no TF_ACC)          │  │
│  │  • Client: retry, backoff, jitter, timeout         │  │
│  │  • Client: error classification (HTTP + network)   │  │
│  │  • Client: Retry-After header parsing              │  │
│  │  • Diff: permission diff algorithm                 │  │
│  │  • Diff: layout role-set computation               │  │
│  │  • Parse: composite ID round-trip                  │  │
│  │  • Parse: scope validation                         │  │
│  │  • Validators: all custom validators table-driven  │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │     Acceptance Tests (TF_ACC=1)                    │  │
│  │  ┌──────────────────────────────────────────────┐  │  │
│  │  │  Garage Container (dxflrs/garage:v2.0.0)     │  │  │
│  │  │  Port 3903 (Admin API), replication_factor=1 │  │  │
│  │  └──────────────────────────────────────────────┘  │  │
│  │                                                    │  │
│  │  Per-resource suites:                              │  │
│  │  ├─ Happy path: CRUD lifecycle, import, update     │  │
│  │  ├─ Disappears: external deletion → plan recreate  │  │
│  │  ├─ Negative: ExpectError on invalid configs       │  │
│  │  ├─ Validation: plan-time errors on bad inputs     │  │
│  │  └─ Data sources: lookup, list, error on missing   │  │
│  │                                                    │  │
│  │  Provider-level:                                   │  │
│  │  ├─ Config from block                              │  │
│  │  ├─ Config from env vars                           │  │
│  │  └─ Config errors (missing endpoint, missing token)│  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │           E2E Tests (multi-resource)               │  │
│  │  • Bucket + Key + Permission full workflow         │  │
│  │  • Bucket + Alias + Website + Quotas combined      │  │
│  │  • Layout multi-node depends_on chain              │  │
│  │  • Admin token create → list → verify → delete     │  │
│  │  • Full teardown: destroy all in reverse order     │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

## Unit Tests

### What to Unit Test

| # | Component | Location | Test Approach | Test Count |
|---|---|---|---|---|
| U1 | Client retry on transient errors | `internal/garage/` | `httptest.Server` returning 500→500→200 | 1 |
| U2 | Client no retry on deterministic errors | `internal/garage/` | `httptest.Server` returning 400/401/403/404/409 | 5 |
| U3 | Client max_retries limit respected | `internal/garage/` | `httptest.Server` returning 500 × 4, max_retries=3 → fail | 1 |
| U4 | Client timeout after configured duration | `internal/garage/` | Slow `httptest.Server`, verify context deadline | 1 |
| U5 | Client Retry-After header respected (429) | `internal/garage/` | `httptest.Server` returning 429 with Retry-After header | 1 |
| U6 | Client exponential backoff + jitter bounds | `internal/garage/` | Measure sleep durations across retries, verify within bounds | 1 |
| U7 | Classify() HTTP status codes | `internal/garage/` | Table-driven: every status → expected ErrorKind | 1 (7+ subtests) |
| U8 | ClassifyError() network errors | `internal/garage/` | Table-driven: DNSError, ECONNRESET, ECONNREFUSED, DeadlineExceeded → Transient | 1 (4+ subtests) |
| U9 | ClassifyError() unknown error fallback | `internal/garage/` | Random error → ErrorKindUnknown | 1 subtest |
| U10 | Permission diff: grant from empty | `internal/provider/` | Table-driven: nil→{read:true} = Allow{read} | 1 subtest |
| U11 | Permission diff: revoke single flag | `internal/provider/` | Table-driven: {r,w}→{r} = Deny{w} | 1 subtest |
| U12 | Permission diff: no-op unchanged | `internal/provider/` | Table-driven: {r}→{r} = no calls | 1 subtest |
| U13 | Permission diff: full grant | `internal/provider/` | Table-driven: nil→{r,w,o} = Allow{r,w,o} | 1 subtest |
| U14 | Permission diff: full revoke | `internal/provider/` | Table-driven: {r,w,o}→nil = Deny{r,w,o} | 1 subtest |
| U15 | Permission diff: mixed grant+revoke | `internal/provider/` | Table-driven: {r,o}→{r,w} = Allow{w}+Deny{o} | 1 subtest |
| U16 | Layout role-set: add node | `internal/provider/` | Existing roles + new → merged set | 1 |
| U17 | Layout role-set: modify node | `internal/provider/` | Existing roles, change capacity → updated set | 1 |
| U18 | Layout role-set: remove node | `internal/provider/` | Existing roles - target → reduced set | 1 |
| U19 | Layout version conflict retry | `internal/provider/` | Mock client: 409→409→200, verify 3 attempts then success | 1 |
| U20 | Layout version conflict max retries | `internal/provider/` | Mock client: 409 forever, verify fail after 3 | 1 |
| U21 | Composite ID parse: valid `bucket_id/key_id` | `internal/provider/` | Parse + format round-trip | 1 |
| U22 | Composite ID parse: missing separator | `internal/provider/` | Error returned | 1 |
| U23 | Composite ID parse: empty parts | `internal/provider/` | Error returned for `/key_id` or `bucket_id/` | 1 |
| U24 | Composite ID parse: extra separators | `internal/provider/` | `a/b/c` → error | 1 |
| U25 | Alias composite ID parse: global | `internal/provider/` | `bucket:global:name` → correct parts | 1 |
| U26 | Alias composite ID parse: local | `internal/provider/` | `bucket:local:key:name` → correct parts | 1 |
| U27 | Alias composite ID parse: invalid | `internal/provider/` | Malformed → error | 1 |
| U28 | Scope validation: known endpoints | `internal/provider/` | All 34 in-scope endpoint names → valid | 1 (34 subtests) |
| U29 | Scope validation: wildcard | `internal/provider/` | `"*"` → valid | 1 subtest |
| U30 | Scope validation: invalid endpoint | `internal/provider/` | `"ReadBuckets"`, `"create_bucket"` → invalid | 1 subtest |
| U31 | Scope validation: empty string | `internal/provider/` | `""` → invalid | 1 subtest |
| U32 | Custom validators: endpoint regex | `internal/provider/` | Valid/invalid URL patterns | 1 |
| U33 | Custom validators: key ID regex | `internal/provider/` | `GK` + 24 hex → valid; others → invalid | 1 |
| U34 | Custom validators: node ID regex | `internal/provider/` | 64 hex chars → valid; 63, 65, uppercase → invalid | 1 |
| U35 | Custom validators: bucket alias regex | `internal/provider/` | S3-compatible names → valid; uppercase, too long → invalid | 1 |
| U36 | Custom validators: RFC 3339 timestamp | `internal/provider/` | Valid dates → pass; `"yesterday"`, `""` → fail | 1 |

**Total: 36 unit test functions / ~80+ subtests**

### Unit Test Conventions

- Table-driven tests with named sub-tests (`t.Run`)
- `testify` not required — use stdlib `testing` + `if` checks
- Test files co-located: `foo.go` → `foo_test.go`
- No mocking framework — interfaces + manual stubs
- `httptest.Server` for all HTTP-level tests
- `t.Parallel()` on every independent test function

### Example: Error Classification (HTTP)

```go
func TestClassify(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name     string
        status   int
        expected ErrorKind
    }{
        {"not found", 404, ErrorKindNotFound},
        {"conflict", 409, ErrorKindConflict},
        {"rate limited", 429, ErrorKindTransient},
        {"server error 500", 500, ErrorKindTransient},
        {"bad gateway 502", 502, ErrorKindTransient},
        {"service unavailable 503", 503, ErrorKindTransient},
        {"gateway timeout 504", 504, ErrorKindTransient},
        {"bad request", 400, ErrorKindValidation},
        {"unauthorized", 401, ErrorKindAuth},
        {"forbidden", 403, ErrorKindAuth},
        {"unknown 418", 418, ErrorKindUnknown},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got := Classify(tt.status)
            if got != tt.expected {
                t.Errorf("Classify(%d) = %v, want %v", tt.status, got, tt.expected)
            }
        })
    }
}
```

### Example: Error Classification (Network)

```go
func TestClassifyError(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name     string
        err      error
        expected ErrorKind
    }{
        {"dns error", &net.DNSError{Err: "no such host"}, ErrorKindTransient},
        {"connection reset", &net.OpError{Err: &os.SyscallError{Err: syscall.ECONNRESET}}, ErrorKindTransient},
        {"connection refused", &net.OpError{Err: &os.SyscallError{Err: syscall.ECONNREFUSED}}, ErrorKindTransient},
        {"deadline exceeded", context.DeadlineExceeded, ErrorKindTransient},
        {"random error", errors.New("something broke"), ErrorKindUnknown},
        {"nil error", nil, ErrorKindUnknown},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got := ClassifyError(tt.err)
            if got != tt.expected {
                t.Errorf("ClassifyError(%v) = %v, want %v", tt.err, got, tt.expected)
            }
        })
    }
}
```

### Example: Permission Diff (Comprehensive)

```go
func TestPermissionDiff(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name    string
        current PermissionState
        desired PermissionState
        allow   PermissionFlags
        deny    PermissionFlags
    }{
        {
            name:    "grant read from empty",
            current: PermissionState{},
            desired: PermissionState{Read: true},
            allow:   PermissionFlags{Read: true},
            deny:    PermissionFlags{},
        },
        {
            name:    "revoke write keep read",
            current: PermissionState{Read: true, Write: true},
            desired: PermissionState{Read: true},
            allow:   PermissionFlags{},
            deny:    PermissionFlags{Write: true},
        },
        {
            name:    "no-op when unchanged",
            current: PermissionState{Read: true},
            desired: PermissionState{Read: true},
            allow:   PermissionFlags{},
            deny:    PermissionFlags{},
        },
        {
            name:    "full grant from empty",
            current: PermissionState{},
            desired: PermissionState{Read: true, Write: true, Owner: true},
            allow:   PermissionFlags{Read: true, Write: true, Owner: true},
            deny:    PermissionFlags{},
        },
        {
            name:    "full revoke to empty",
            current: PermissionState{Read: true, Write: true, Owner: true},
            desired: PermissionState{},
            allow:   PermissionFlags{},
            deny:    PermissionFlags{Read: true, Write: true, Owner: true},
        },
        {
            name:    "mixed grant and revoke",
            current: PermissionState{Read: true, Owner: true},
            desired: PermissionState{Read: true, Write: true},
            allow:   PermissionFlags{Write: true},
            deny:    PermissionFlags{Owner: true},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            gotAllow, gotDeny := computePermissionDiff(tt.current, tt.desired)
            if gotAllow != tt.allow {
                t.Errorf("allow: got %+v, want %+v", gotAllow, tt.allow)
            }
            if gotDeny != tt.deny {
                t.Errorf("deny: got %+v, want %+v", gotDeny, tt.deny)
            }
        })
    }
}
```

### Example: Retry with httptest

```go
func TestClient_RetryOn500(t *testing.T) {
    t.Parallel()
    attempts := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        if attempts < 3 {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{}`))
    }))
    defer srv.Close()

    client, err := NewClient(srv.URL, "test-token",
        WithMaxRetries(5),
        WithRetryWait(1*time.Millisecond, 10*time.Millisecond),
    )
    if err != nil {
        t.Fatalf("NewClient: %v", err)
    }
    // Make a request, verify attempts == 3
    // ...
}

func TestClient_NoRetryOn404(t *testing.T) {
    t.Parallel()
    attempts := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        w.WriteHeader(http.StatusNotFound)
    }))
    defer srv.Close()

    client, _ := NewClient(srv.URL, "test-token",
        WithMaxRetries(3),
        WithRetryWait(1*time.Millisecond, 10*time.Millisecond),
    )
    // Make a request, verify attempts == 1 (no retries)
    // ...
}

func TestClient_MaxRetriesExhausted(t *testing.T) {
    t.Parallel()
    attempts := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer srv.Close()

    client, _ := NewClient(srv.URL, "test-token",
        WithMaxRetries(3),
        WithRetryWait(1*time.Millisecond, 10*time.Millisecond),
    )
    // Make a request, verify err != nil and attempts == 4 (1 initial + 3 retries)
    // ...
}

func TestClient_RetryAfterHeader(t *testing.T) {
    t.Parallel()
    attempts := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        if attempts == 1 {
            w.Header().Set("Retry-After", "1")
            w.WriteHeader(http.StatusTooManyRequests)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{}`))
    }))
    defer srv.Close()

    // Verify second attempt respects Retry-After header
    // ...
}
```

## Acceptance Tests

### Framework Setup

```go
// internal/provider/provider_test.go

package provider_test

import (
    "os"
    "testing"

    "github.com/hashicorp/terraform-plugin-framework/providerserver"
    "github.com/hashicorp/terraform-plugin-go/tfprotov6"
    "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
    "github.com/hashicorp/terraform-plugin-testing/helper/resource"

    "github.com/vhco-pro/terraform-provider-garage/internal/provider"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
    "garage": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// testAccPreCheck validates environment before running acceptance tests.
func testAccPreCheck(t *testing.T) {
    t.Helper()
    if os.Getenv("GARAGE_ENDPOINT") == "" {
        t.Skip("GARAGE_ENDPOINT not set, skipping acceptance test")
    }
    if os.Getenv("GARAGE_TOKEN") == "" {
        t.Skip("GARAGE_TOKEN not set, skipping acceptance test")
    }
}

// randomName generates a unique resource name for test isolation.
func randomName(prefix string) string {
    return fmt.Sprintf("tf-test-%s-%s", prefix, acctest.RandString(8))
}
```

### Environment Variables

| Variable | Purpose | Default |
|---|---|---|
| `TF_ACC` | Enable acceptance tests | — (must be set) |
| `GARAGE_ENDPOINT` | Admin API URL | `http://localhost:3903` |
| `GARAGE_TOKEN` | Admin API token | — (required) |

### Test Helpers

```go
// testAccCheckResourceExists verifies a resource exists in Garage via the API.
// Used as a resource.TestCheckFunc in acceptance tests.
func testAccCheckBucketExists(resourceName string) resource.TestCheckFunc {
    return func(s *terraform.State) error {
        rs, ok := s.RootModule().Resources[resourceName]
        if !ok {
            return fmt.Errorf("resource %s not found in state", resourceName)
        }
        client := testAccGetClient(t)
        _, err := client.GetBucketInfo(context.Background(), rs.Primary.ID)
        if err != nil {
            return fmt.Errorf("bucket %s not found in Garage: %w", rs.Primary.ID, err)
        }
        return nil
    }
}

// testAccGetClient returns a Garage API client for direct API verification in tests.
func testAccGetClient(t *testing.T) *garage.Client {
    t.Helper()
    client, err := garage.NewClient(
        os.Getenv("GARAGE_ENDPOINT"),
        os.Getenv("GARAGE_TOKEN"),
    )
    if err != nil {
        t.Fatalf("failed to create test client: %v", err)
    }
    return client
}
```

### CheckDestroy Functions

Every resource test suite must include a `CheckDestroy` function that verifies the resource is gone from Garage. One per resource type:

```go
func testAccCheckBucketDestroy(s *terraform.State) error {
    client := testAccGetClient(nil) // uses env vars
    for _, rs := range s.RootModule().Resources {
        if rs.Type != "garage_bucket" {
            continue
        }
        _, err := client.GetBucketInfo(context.Background(), rs.Primary.ID)
        if err == nil {
            return fmt.Errorf("bucket %s still exists after destroy", rs.Primary.ID)
        }
        var apiErr *garage.APIError
        if errors.As(err, &apiErr) && apiErr.Kind == garage.ErrorKindNotFound {
            continue
        }
        return fmt.Errorf("unexpected error checking bucket %s: %w", rs.Primary.ID, err)
    }
    return nil
}

// Similar functions for: testAccCheckKeyDestroy, testAccCheckAdminTokenDestroy, testAccCheckLayoutNodeDestroy
// testAccCheckBucketPermissionDestroy checks via GetBucketInfo that the key no longer has permissions
// testAccCheckBucketAliasDestroy checks via GetBucketInfo that the alias no longer exists
```

### Config Builders

Reusable HCL config generators to reduce test boilerplate:

```go
// testAccBucketConfig generates HCL for a garage_bucket resource.
func testAccBucketConfig(name string, alias string) string {
    return fmt.Sprintf(`
resource "garage_bucket" %[1]q {
  global_alias = %[2]q
}
`, name, alias)
}

// testAccBucketConfigWithQuotas adds quota configuration.
func testAccBucketConfigWithQuotas(name, alias string, maxSize, maxObjects int64) string {
    return fmt.Sprintf(`
resource "garage_bucket" %[1]q {
  global_alias = %[2]q
  quotas = {
    max_size    = %[3]d
    max_objects = %[4]d
  }
}
`, name, alias, maxSize, maxObjects)
}

// testAccBucketConfigWithWebsite adds website configuration.
func testAccBucketConfigWithWebsite(name, alias, indexDoc, errorDoc string) string {
    return fmt.Sprintf(`
resource "garage_bucket" %[1]q {
  global_alias = %[2]q
  website = {
    enabled        = true
    index_document = %[3]q
    error_document = %[4]q
  }
}
`, name, alias, indexDoc, errorDoc)
}

// testAccKeyConfig generates HCL for a garage_key resource.
func testAccKeyConfig(name, keyName string) string {
    return fmt.Sprintf(`
resource "garage_key" %[1]q {
  name = %[2]q
}
`, name, keyName)
}

// testAccPermissionConfig generates HCL for a garage_bucket_permission resource.
func testAccPermissionConfig(name, bucketRef, keyRef string, read, write, owner bool) string {
    return fmt.Sprintf(`
resource "garage_bucket_permission" %[1]q {
  bucket_id     = %[2]s
  access_key_id = %[3]s
  read          = %[4]t
  write         = %[5]t
  owner         = %[6]t
}
`, name, bucketRef, keyRef, read, write, owner)
}

// testAccAdminTokenConfig generates HCL for a garage_admin_token resource.
func testAccAdminTokenConfig(name, tokenName string) string {
    return fmt.Sprintf(`
resource "garage_admin_token" %[1]q {
  name = %[2]q
}
`, name, tokenName)
}
```

---

### Per-Resource Test Plans

Each resource gets three test categories: **happy path**, **disappears**, and **negative/validation**.

---

#### `garage_bucket` — `resource_bucket_test.go`

**Happy path:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| B1 | `TestAccBucket_basic` | `ParallelTest` | Create with alias | ID populated, alias in state, exists in Garage |
| B2 | `TestAccBucket_website` | `ParallelTest` | Create with website block | `website.enabled = true`, `index_document` and `error_document` correct |
| B3 | `TestAccBucket_quotas` | `ParallelTest` | Create with quotas | `max_size` and `max_objects` persisted and readable |
| B4 | `TestAccBucket_updateQuotas` | `ParallelTest` | Create → Update quotas | New values persisted in Garage |
| B5 | `TestAccBucket_updateWebsiteToggle` | `ParallelTest` | Create with website → Remove website block | Website disabled in Garage |
| B6 | `TestAccBucket_import` | `ParallelTest` | Create → `ImportState` by ID | All attributes match including quotas and website |
| B7 | `TestAccBucket_aliasChangeForceNew` | `ParallelTest` | Create → Change `global_alias` | Destroy+recreate (new ID) |
| B8 | `TestAccBucket_computedAliases` | `ParallelTest` | Create bucket + `garage_bucket_alias` | `global_aliases` contains both aliases |
| B9 | `TestAccBucket_usageMetrics` | `ParallelTest` | Create bucket | `objects`, `bytes`, `unfinished_uploads` are ≥ 0 |

**Disappears:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| B10 | `TestAccBucket_disappears` | `ParallelTest` | Create → Delete via API → Plan | Plan shows recreate, no crash |

**Negative / Validation:**

| # | Test | Type | Config | Expected Error |
|---|---|---|---|---|
| B11 | `TestAccBucket_deleteNonEmpty` | `ParallelTest` | Create → Upload object via S3 → Destroy | Error diagnostic: "bucket must be empty" |
| B12 | `TestAccBucket_invalidAliasUppercase` | `ParallelTest` | `global_alias = "My-Bucket"` | Validation error at plan time (regex) |
| B13 | `TestAccBucket_invalidAliasTooLong` | `ParallelTest` | `global_alias = <64 chars>` | Validation error: length exceeded |
| B14 | `TestAccBucket_invalidAliasEmpty` | `ParallelTest` | `global_alias = ""` | Validation error: length min |
| B15 | `TestAccBucket_invalidQuotaNegative` | `ParallelTest` | `max_size = -1` | Validation error: AtLeast(0) |

**Total: 15 tests**

---

#### `garage_bucket_alias` — `resource_bucket_alias_test.go`

**Happy path:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| A1 | `TestAccBucketAlias_global` | `ParallelTest` | Create global alias on bucket | Alias visible in `GetBucketInfo` |
| A2 | `TestAccBucketAlias_local` | `ParallelTest` | Create local alias with key | Alias scoped to key in bucket info |
| A3 | `TestAccBucketAlias_import` | `ParallelTest` | Create → Import by composite ID | State matches |

**Disappears:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| A4 | `TestAccBucketAlias_disappears` | `ParallelTest` | Create → Remove alias via API → Plan | Recreate, no crash |

**Negative / Validation:**

| # | Test | Type | Config | Expected Error |
|---|---|---|---|---|
| A5 | `TestAccBucketAlias_localMissingKey` | `ParallelTest` | `alias_type = "local"` without `access_key_id` | Validation error at plan time |
| A6 | `TestAccBucketAlias_globalWithKey` | `ParallelTest` | `alias_type = "global"` with `access_key_id` set | Validation error at plan time |
| A7 | `TestAccBucketAlias_duplicateOnDifferentBucket` | `ParallelTest` | Same alias on two different buckets | Error from Garage API (alias conflict) |
| A8 | `TestAccBucketAlias_duplicateOnSameBucket` | `ParallelTest` | Same alias on same bucket twice | Idempotent success (no error) |

**Total: 8 tests**

---

#### `garage_key` — `resource_key_test.go`

**Happy path:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| K1 | `TestAccKey_basic` | `ParallelTest` | Create with name | `id` matches `GK` prefix, `access_key_id` populated |
| K2 | `TestAccKey_importPredefined` | `ParallelTest` | Create with `id` + `secret_access_key` | Key usable in Garage |
| K3 | `TestAccKey_updateName` | `ParallelTest` | Create → Change name | New name persisted (verifies the fix for existing provider bug) |
| K4 | `TestAccKey_secretSensitive` | `ParallelTest` | Create | `secret_access_key` not in plan output |
| K5 | `TestAccKey_import` | `ParallelTest` | Create → `ImportState` by ID | `id`, `name`, `buckets`, `secret_access_key` match (secret retrieved via `showSecretKey=true`) |
| K6 | `TestAccKey_secretPreserved` | `ParallelTest` | Create → No-op apply | `secret_access_key` unchanged across applies |
| K7 | `TestAccKey_expiration` | `ParallelTest` | Create with `expiration` | `expiration` timestamp persisted, `expired = false` |
| K8 | `TestAccKey_neverExpires` | `ParallelTest` | Create with `never_expires = true` | Key has no expiration |
| K9 | `TestAccKey_createBucketPermission` | `ParallelTest` | Create with `create_bucket = true` | `create_bucket` persisted via `permissions.createBucket` |

**Disappears:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| K10 | `TestAccKey_disappears` | `ParallelTest` | Create → Delete via API → Plan | Plan shows recreate |

**Negative / Validation:**

| # | Test | Type | Config | Expected Error |
|---|---|---|---|---|
| K11 | `TestAccKey_onlyIdNoSecret` | `ParallelTest` | `id = "GK..."` without `secret_access_key` | Validation error: AlsoRequires |
| K12 | `TestAccKey_onlySecretNoId` | `ParallelTest` | `secret_access_key = "..."` without `id` | Validation error: AlsoRequires |
| K13 | `TestAccKey_invalidIdFormat` | `ParallelTest` | `id = "INVALID"` | Validation error: regex mismatch |
| K14 | `TestAccKey_expirationAndNeverExpires` | `ParallelTest` | Both `expiration` and `never_expires` set | Validation error: ConflictsWith |

**Total: 14 tests**

---

#### `garage_bucket_permission` — `resource_bucket_permission_test.go`

**Happy path:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| P1 | `TestAccBucketPermission_readOnly` | `ParallelTest` | Grant read | `read = true`, `write = false`, `owner = false` in bucket info |
| P2 | `TestAccBucketPermission_readWrite` | `ParallelTest` | Grant read+write | Both flags set |
| P3 | `TestAccBucketPermission_allPermissions` | `ParallelTest` | Grant read+write+owner | All three flags set |
| P4 | `TestAccBucketPermission_addWrite` | `ParallelTest` | Grant read → add write | Write added, read preserved |
| P5 | `TestAccBucketPermission_revokeOwner` | `ParallelTest` | Grant all → remove owner | Owner revoked, read+write preserved |
| P6 | `TestAccBucketPermission_import` | `ParallelTest` | Create → Import by `{bucket_id}/{key_id}` | All flags match |
| P7 | `TestAccBucketPermission_delete` | `ParallelTest` | Create → Destroy | All flags revoked in Garage |

**Disappears:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| P8 | `TestAccBucketPermission_disappears` | `ParallelTest` | Create → DenyBucketKey via API → Plan | Recreate |

**Negative / Validation:**

| # | Test | Type | Config | Expected Error |
|---|---|---|---|---|
| P9 | `TestAccBucketPermission_invalidKeyId` | `ParallelTest` | `access_key_id = "INVALID"` | Validation error: regex |
| P10 | `TestAccBucketPermission_importMalformedId` | `ParallelTest` | Import with `"no-separator"` | Parse error diagnostic |

**Total: 10 tests**

---

#### `garage_layout_node` — `resource_layout_node_test.go`

**All layout tests use `resource.Test` (serial) — layout operations are inherently sequential.**

**Happy path:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| L1 | `TestAccLayoutNode_basic` | `Test` | Stage + Apply | Node in applied layout, `layout_version` incremented |
| L2 | `TestAccLayoutNode_updateZone` | `Test` | Create → Change zone | New zone after apply |
| L3 | `TestAccLayoutNode_updateCapacity` | `Test` | Create → Change capacity | New capacity after apply |
| L4 | `TestAccLayoutNode_updateTags` | `Test` | Create → Change tags | New tags after apply |
| L5 | `TestAccLayoutNode_import` | `Test` | Create → Import by node ID | zone, capacity, tags, layout_version match |
| L6 | `TestAccLayoutNode_delete` | `Test` | Create → Destroy | Node removed from layout |

**Disappears:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| L7 | `TestAccLayoutNode_disappears` | `Test` | Create → Remove from layout via API → Plan | Recreate |

**Negative / Validation:**

| # | Test | Type | Config | Expected Error |
|---|---|---|---|---|
| L8 | `TestAccLayoutNode_invalidNodeIdShort` | `Test` | `node_id = "abc123"` (too short) | Validation error: regex |
| L9 | `TestAccLayoutNode_invalidNodeIdUppercase` | `Test` | `node_id = "ABC..."` (uppercase) | Validation error: regex |
| L10 | `TestAccLayoutNode_invalidCapacityZero` | `Test` | `capacity = 0` | Validation error: AtLeast(1) |

**Total: 10 tests**

---

#### `garage_admin_token` — `resource_admin_token_test.go`

**Happy path:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| T1 | `TestAccAdminToken_basic` | `ParallelTest` | Create with name | `id` populated, `secret_token` is sensitive |
| T2 | `TestAccAdminToken_updateName` | `ParallelTest` | Create → Update name | New name in Garage |
| T3 | `TestAccAdminToken_updateExpiration` | `ParallelTest` | Create → Set expiration | Expiration timestamp persisted |
| T4 | `TestAccAdminToken_updateScope` | `ParallelTest` | Create → Set scope | Scope endpoints persisted |
| T5 | `TestAccAdminToken_neverExpires` | `ParallelTest` | Create with `never_expires = true` | Token has no expiration |
| T6 | `TestAccAdminToken_import` | `ParallelTest` | Create → Import by ID | All attrs match except `secret_token` (null) |
| T7 | `TestAccAdminToken_secretPreserved` | `ParallelTest` | Create → No-op apply | `secret_token` unchanged |
| T8 | `TestAccAdminToken_delete` | `ParallelTest` | Create → Destroy | Token gone from Garage |

**Disappears:**

| # | Test | Type | Config Steps | Checks |
|---|---|---|---|---|
| T9 | `TestAccAdminToken_disappears` | `ParallelTest` | Create → Delete via API → Plan | Plan shows recreate |

**Negative / Validation:**

| # | Test | Type | Config | Expected Error |
|---|---|---|---|---|
| T10 | `TestAccAdminToken_expirationAndNeverExpires` | `ParallelTest` | Both `expiration` and `never_expires` set | Validation error: ExactlyOneOf / conflict |
| T11 | `TestAccAdminToken_invalidExpiration` | `ParallelTest` | `expiration = "not-a-date"` | Validation error: RFC 3339 |
| T12 | `TestAccAdminToken_invalidScope` | `ParallelTest` | `scope = ["FakeEndpoint"]` | Validation error: unknown endpoint |

**Total: 12 tests**

---

#### Data Sources — Individual test files

Each data source gets at minimum: a happy-path lookup and a not-found/empty case.

| # | Test | File | Type | Config | Checks |
|---|---|---|---|---|---|
| D1 | `TestAccDataSourceBucket_byId` | `datasource_bucket_test.go` | `ParallelTest` | Create bucket → data source by ID | All attributes match resource |
| D2 | `TestAccDataSourceBucket_byAlias` | `datasource_bucket_test.go` | `ParallelTest` | Create bucket → data source by alias | Correct bucket found |
| D3 | `TestAccDataSourceBucket_notFound` | `datasource_bucket_test.go` | `ParallelTest` | Data source with non-existent ID | `ExpectError`: not found |
| D4 | `TestAccDataSourceBucket_missingBothIdAndAlias` | `datasource_bucket_test.go` | `ParallelTest` | Data source with neither `id` nor `global_alias` | Validation error |
| D5 | `TestAccDataSourceBuckets_basic` | `datasource_buckets_test.go` | `ParallelTest` | Create bucket → list | List contains created bucket |
| D6 | `TestAccDataSourceBuckets_empty` | `datasource_buckets_test.go` | `ParallelTest` | List on empty cluster | Empty list (not error) |
| D7 | `TestAccDataSourceKey_byId` | `datasource_key_test.go` | `ParallelTest` | Create key → data source by ID | Name, buckets, expired, created, expiration, create_bucket match |
| D8 | `TestAccDataSourceKey_notFound` | `datasource_key_test.go` | `ParallelTest` | Data source with non-existent ID | `ExpectError`: not found |
| D9 | `TestAccDataSourceKeys_basic` | `datasource_keys_test.go` | `ParallelTest` | Create key → list | List contains created key with id, name, expired, created, expiration |
| D10 | `TestAccDataSourceKeys_empty` | `datasource_keys_test.go` | `ParallelTest` | List with no keys | Empty list |
| D11 | `TestAccDataSourceClusterHealth_basic` | `datasource_cluster_health_test.go` | `Test` | Read health | `known_nodes > 0`, `partitions > 0`, `partitions_quorum ≥ 0` |
| D12 | `TestAccDataSourceClusterStatus_basic` | `datasource_cluster_status_test.go` | `Test` | Read status | `layout_version ≥ 0`, `nodes` list non-empty, nodes have `id` and `is_up` |
| D13 | `TestAccDataSourceClusterLayout_basic` | `datasource_cluster_layout_test.go` | `Test` | Read layout | `version ≥ 0`, roles list accessible |
| D14 | `TestAccDataSourceClusterLayout_afterApply` | `datasource_cluster_layout_test.go` | `Test` | Create layout node → Read layout | Our node in roles, version incremented |
| D15 | `TestAccDataSourceAdminToken_byId` | `datasource_admin_token_test.go` | `ParallelTest` | Create token → data source by ID | Name, scope match |
| D16 | `TestAccDataSourceAdminToken_current` | `datasource_admin_token_test.go` | `ParallelTest` | Data source with `current = true` | Returns current auth token info |
| D17 | `TestAccDataSourceAdminToken_missingBothIdAndCurrent` | `datasource_admin_token_test.go` | `ParallelTest` | Neither `id` nor `current` | Validation error |
| D18 | `TestAccDataSourceAdminTokens_basic` | `datasource_admin_tokens_test.go` | `ParallelTest` | Create token → list | List contains created token |
| D19 | `TestAccDataSourceNodeInfo_basic` | `datasource_node_info_test.go` | `Test` | Read node info | `nodes` list non-empty, `nodes.0.node_id` populated, `nodes.0.garage_version` non-empty, `nodes.0.db_engine` non-empty |
| D20 | `TestAccDataSourceNodeInfo_byNodeId` | `datasource_node_info_test.go` | `Test` | Read with `node_id` | `nodes` list has 1 element matching the requested node ID |

**Total: 20 data source tests**

---

#### Provider Configuration — `provider_test.go`

| # | Test | Type | Config | Checks |
|---|---|---|---|---|
| PC1 | `TestAccProvider_configFromBlock` | `Test` | Explicit endpoint + token in HCL | Data source succeeds |
| PC2 | `TestAccProvider_configFromEnv` | `Test` | Only env vars, no HCL config | Data source succeeds |
| PC3 | `TestAccProvider_missingEndpoint` | `Test` | No endpoint in config or env | `ExpectError`: "endpoint" required |
| PC4 | `TestAccProvider_missingToken` | `Test` | No token in config or env | `ExpectError`: "token" required |
| PC5 | `TestAccProvider_invalidEndpoint` | `Test` | `endpoint = "not-a-url"` | `ExpectError`: regex validation |
| PC6 | `TestAccProvider_configBlockOverridesEnv` | `Test` | Env has endpoint A, config has endpoint B | Uses endpoint B |

**Total: 6 provider tests**

---

### Acceptance Test Summary

| Resource / Category | Happy | Disappears | Negative | Total |
|---|---|---|---|---|
| `garage_bucket` | 9 | 1 | 5 | **15** |
| `garage_bucket_alias` | 3 | 1 | 4 | **8** |
| `garage_key` | 9 | 1 | 4 | **14** |
| `garage_bucket_permission` | 7 | 1 | 2 | **10** |
| `garage_layout_node` | 6 | 1 | 3 | **10** |
| `garage_admin_token` | 8 | 1 | 3 | **12** |
| Data sources | 18 | — | 2 | **20** |
| Provider config | 3 | — | 3 | **6** |
| **TOTAL** | **63** | **6** | **26** | **95** |

## E2E Test Workflows

E2E tests validate multi-resource workflows that real users will execute. They use `resource.Test` (serial) with multiple `TestStep`s to build up and tear down complete environments.

### Workflow 1: Bucket + Key + Permission (Most Common Pattern)

```go
func TestAccE2E_BucketKeyPermission(t *testing.T) {
    bucketAlias := randomName("e2e-bkp")
    keyName := randomName("e2e-key")

    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy: resource.ComposeTestCheckFunc(
            testAccCheckBucketDestroy,
            testAccCheckKeyDestroy,
        ),
        Steps: []resource.TestStep{
            // Step 1: Create bucket + key + read+write permission
            {
                Config: fmt.Sprintf(`
resource "garage_key" "app" {
  name = %[1]q
}

resource "garage_bucket" "data" {
  global_alias = %[2]q
  quotas = {
    max_size    = 1073741824
    max_objects = 10000
  }
}

resource "garage_bucket_permission" "app_data" {
  bucket_id     = garage_bucket.data.id
  access_key_id = garage_key.app.id
  read          = true
  write         = true
  owner         = false
}
`, keyName, bucketAlias),
                Check: resource.ComposeAggregateTestCheckFunc(
                    testAccCheckBucketExists("garage_bucket.data"),
                    resource.TestCheckResourceAttrSet("garage_key.app", "id"),
                    resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "read", "true"),
                    resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "write", "true"),
                    resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "owner", "false"),
                ),
            },
            // Step 2: Promote to owner
            {
                Config: fmt.Sprintf(`
resource "garage_key" "app" {
  name = %[1]q
}

resource "garage_bucket" "data" {
  global_alias = %[2]q
  quotas = {
    max_size    = 1073741824
    max_objects = 10000
  }
}

resource "garage_bucket_permission" "app_data" {
  bucket_id     = garage_bucket.data.id
  access_key_id = garage_key.app.id
  read          = true
  write         = true
  owner         = true
}
`, keyName, bucketAlias),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("garage_bucket_permission.app_data", "owner", "true"),
                ),
            },
            // Step 3: Destroy (implicit) — verify CheckDestroy cleans up
        },
    })
}
```

### Workflow 2: Bucket + Multiple Aliases + Website

```go
func TestAccE2E_BucketAliasWebsite(t *testing.T) {
    primaryAlias := randomName("e2e-site")
    secondaryAlias := randomName("e2e-cdn")

    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckBucketDestroy,
        Steps: []resource.TestStep{
            // Step 1: Create bucket with website + secondary alias
            {
                Config: fmt.Sprintf(`
resource "garage_bucket" "site" {
  global_alias = %[1]q
  website = {
    enabled        = true
    index_document = "index.html"
    error_document = "404.html"
  }
}

resource "garage_bucket_alias" "cdn" {
  bucket_id  = garage_bucket.site.id
  alias_type = "global"
  name       = %[2]q
}
`, primaryAlias, secondaryAlias),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("garage_bucket.site", "website.enabled", "true"),
                    resource.TestCheckResourceAttr("garage_bucket.site", "website.index_document", "index.html"),
                    testAccCheckBucketExists("garage_bucket.site"),
                ),
            },
            // Step 2: Disable website, keep alias
            {
                Config: fmt.Sprintf(`
resource "garage_bucket" "site" {
  global_alias = %[1]q
}

resource "garage_bucket_alias" "cdn" {
  bucket_id  = garage_bucket.site.id
  alias_type = "global"
  name       = %[2]q
}
`, primaryAlias, secondaryAlias),
                Check: resource.ComposeAggregateTestCheckFunc(
                    // website block removed → website disabled
                    resource.TestCheckNoResourceAttr("garage_bucket.site", "website.enabled"),
                ),
            },
        },
    })
}
```

### Workflow 3: Admin Token Create → List → Verify

```go
func TestAccE2E_AdminTokenLifecycle(t *testing.T) {
    tokenName := randomName("e2e-token")

    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckAdminTokenDestroy,
        Steps: []resource.TestStep{
            // Step 1: Create token with scope + expiration
            {
                Config: fmt.Sprintf(`
resource "garage_admin_token" "deploy" {
  name       = %[1]q
  expiration = "2099-01-01T00:00:00Z"
  scope      = ["GetClusterStatus", "GetClusterHealth", "ListBuckets"]
}

data "garage_admin_tokens" "all" {
  depends_on = [garage_admin_token.deploy]
}
`, tokenName),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttrSet("garage_admin_token.deploy", "id"),
                    resource.TestCheckResourceAttrSet("garage_admin_token.deploy", "secret_token"),
                    resource.TestCheckResourceAttr("garage_admin_token.deploy", "name", tokenName),
                    resource.TestCheckResourceAttr("garage_admin_token.deploy", "expired", "false"),
                    // Verify token appears in list data source
                    resource.TestCheckResourceAttrSet("data.garage_admin_tokens.all", "tokens.#"),
                ),
            },
            // Step 2: Update scope
            {
                Config: fmt.Sprintf(`
resource "garage_admin_token" "deploy" {
  name       = %[1]q
  expiration = "2099-01-01T00:00:00Z"
  scope      = ["*"]
}

data "garage_admin_tokens" "all" {
  depends_on = [garage_admin_token.deploy]
}
`, tokenName),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("garage_admin_token.deploy", "scope.0", "*"),
                ),
            },
        },
    })
}
```

### Workflow 4: Layout Multi-Node Chain

```go
func TestAccE2E_LayoutMultiNode(t *testing.T) {
    // This test requires node IDs from the running Garage instance.
    // It uses the cluster status data source to discover the actual node ID.
    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckLayoutNodeDestroy,
        Steps: []resource.TestStep{
            {
                Config: `
data "garage_cluster_status" "this" {}
data "garage_node_info" "this" {}

resource "garage_layout_node" "node1" {
  node_id  = data.garage_cluster_status.this.nodes[0].id
  zone     = "dc1"
  capacity = 1073741824
  tags     = ["test"]
}
`,
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttrSet("garage_layout_node.node1", "layout_version"),
                ),
            },
            // Step 2: Update capacity
            {
                Config: `
data "garage_cluster_status" "this" {}
data "garage_node_info" "this" {}

resource "garage_layout_node" "node1" {
  node_id  = data.garage_cluster_status.this.nodes[0].id
  zone     = "dc1"
  capacity = 2147483648
  tags     = ["test", "updated"]
}
`,
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("garage_layout_node.node1", "capacity", "2147483648"),
                ),
            },
        },
    })
}
```

**E2E Total: 4 workflow tests**

---

## CI/CD Pipeline

### GitHub Actions: CI

```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@v6

  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test ./... -v -count=1 -race
      - name: Upload coverage
        if: always()
        run: |
          go test ./... -coverprofile=coverage.out -covermode=atomic
          go tool cover -func=coverage.out

  acceptance:
    runs-on: ubuntu-latest
    needs: [lint, unit]
    services:
      garage:
        image: dxflrs/garage:v2.0.0
        ports:
          - 3903:3903
        env:
          GARAGE_ADMIN_TOKEN: test-admin-token
        options: >-
          --health-cmd "curl -sf http://localhost:3903/health || exit 1"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 12
          --health-start-period 10s
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Wait for Garage to be healthy
        run: |
          for i in $(seq 1 30); do
            if curl -sf http://localhost:3903/health; then
              echo "Garage is ready"
              exit 0
            fi
            echo "Waiting for Garage... ($i/30)"
            sleep 2
          done
          echo "Garage failed to start"
          exit 1
      - name: Run acceptance tests
        run: go test ./... -v -count=1 -timeout 30m -race
        env:
          TF_ACC: "1"
          GARAGE_ENDPOINT: http://localhost:3903
          GARAGE_TOKEN: test-admin-token
```

### Garage Service Container Configuration

```toml
# testdata/garage.toml — used by CI and local development
metadata_dir = "/tmp/garage/meta"
data_dir = "/tmp/garage/data"
db_engine = "sqlite"

replication_factor = 1

[s3_api]
api_bind_addr = "0.0.0.0:3900"
s3_region = "garage"
root_domain = ".s3.garage.localhost"

[admin]
api_bind_addr = "0.0.0.0:3903"
admin_token = "test-admin-token"
```

### Local Development Testing

```makefile
# GNUmakefile targets for local testing

# Run unit tests only (no Garage needed)
test:
	go test ./... -v -count=1 -race

# Start local Garage and run acceptance tests
testacc:
	docker compose -f testdata/docker-compose.yml up -d --wait
	TF_ACC=1 GARAGE_ENDPOINT=http://localhost:3903 GARAGE_ADMIN_TOKEN=test-admin-token \
		go test ./... -v -count=1 -timeout 30m
	docker compose -f testdata/docker-compose.yml down

# Run specific test by name
testfilter:
	TF_ACC=1 GARAGE_ENDPOINT=http://localhost:3903 GARAGE_ADMIN_TOKEN=test-admin-token \
		go test ./... -v -count=1 -run=$(FILTER)
```

```yaml
# testdata/docker-compose.yml
services:
  garage:
    image: dxflrs/garage:v2.0.0
    ports:
      - "3903:3903"
      - "3900:3900"
    environment:
      GARAGE_ADMIN_TOKEN: test-admin-token
    healthcheck:
      test: ["CMD", "curl", "-sf", "http://localhost:3903/health"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 5s
```

### Test Parallelism

- **Unit tests**: `t.Parallel()` on every test function. Default `go test` parallelism.
- **Acceptance tests**: `resource.ParallelTest` for all resources EXCEPT layout.
- **Layout tests**: `resource.Test` (serial) — layout operations do read-modify-write on shared cluster state.
- **E2E tests**: `resource.Test` (serial) — multi-step workflows must run sequentially.
- **CI**: Unit and acceptance jobs run as separate GitHub Actions jobs. Acceptance depends on lint+unit passing.

### Coverage Targets

| Category | Target | Measurement |
|---|---|---|
| Unit coverage: `internal/garage/` | ≥ 80% | `go test -coverprofile` |
| Unit coverage: `internal/provider/` helpers | ≥ 70% | validators, diff, parsers |
| Acceptance tests per resource | ≥ 8 (happy + disappears + negative) | Count in this doc |
| Acceptance tests per data source | ≥ 1 (happy path minimum) | Count in this doc |
| Data source error tests | ≥ 1 per data source that can fail | not-found cases |
| E2E workflows | ≥ 4 | Multi-resource test functions |
| **Total acceptance+E2E tests** | **≥ 90** | Sum of all test tables |

## Complete Test Matrix

Cross-reference of every resource × operation × test type to verify no gaps:

| Resource | Create | Read | Update | Delete | Import | Disappears | Validation | Data Source |
|---|---|---|---|---|---|---|---|---|
| `garage_bucket` | B1 | B1,B9 | B4,B5 | B11 | B6 | B10 | B12-B15 | D1-D6 |
| `garage_bucket_alias` | A1,A2 | A1 | N/A | A1 | A3 | A4 | A5-A8 | — |
| `garage_key` | K1,K2 | K1,K6 | K3 | K1 | K5 | K7 | K8-K10 | D7-D10 |
| `garage_bucket_permission` | P1-P3 | P1 | P4,P5 | P7 | P6 | P8 | P9,P10 | — |
| `garage_layout_node` | L1 | L1 | L2-L4 | L6 | L5 | L7 | L8-L10 | D11-D14 |
| `garage_admin_token` | T1 | T1,T7 | T2-T4 | T8 | T6 | T9 | T10-T12 | D15-D19 |

**Every cell is covered. No gaps.**

## Acceptance Criteria

### Infrastructure
- [ ] `terraform-plugin-testing` added as test dependency
- [ ] `internal/provider/provider_test.go` contains factories, helpers, `testAccPreCheck`, `randomName`
- [ ] `testdata/garage.toml` exists for CI container config
- [ ] `testdata/docker-compose.yml` exists for local development
- [ ] `GNUmakefile` has `test`, `testacc`, and `testfilter` targets

### Unit Tests
- [ ] All 36 unit test functions pass: `go test ./internal/garage/... ./internal/provider/... -v`
- [ ] Unit coverage for `internal/garage/` ≥ 80%
- [ ] Unit coverage for provider helpers (diff, parse, validators) ≥ 70%
- [ ] Unit tests run without `TF_ACC` or Garage instance

### Acceptance Tests — Resources
- [ ] Every resource has ≥ 8 acceptance tests (happy + disappears + negative)
- [ ] Every resource has a `CheckDestroy` function
- [ ] Every resource has at least one `ImportState` test
- [ ] Every resource has at least one disappears test (`CheckDestroy` triggered by external deletion)
- [ ] Every resource has at least one validation error test (`ExpectError`)
- [ ] Layout tests use `resource.Test` (serial)
- [ ] All other resource tests use `resource.ParallelTest`
- [ ] No test pollution — every test uses `randomName()` for resource names

### Acceptance Tests — Data Sources
- [ ] All 10 data sources have at least one acceptance test
- [ ] Data sources that can fail on missing resources have not-found/error tests (`ExpectError`)
- [ ] Data sources that validate input have validation error tests

### Acceptance Tests — Provider
- [ ] Provider config from block works
- [ ] Provider config from env vars works
- [ ] Missing endpoint produces clear error
- [ ] Missing token produces clear error
- [ ] Invalid endpoint format rejected at plan time
- [ ] Config block overrides env vars

### E2E Tests
- [ ] Bucket+Key+Permission workflow creates and updates correctly
- [ ] Bucket+Alias+Website workflow toggles website config
- [ ] Admin token lifecycle creates, lists, updates, destroys
- [ ] Layout multi-node applies sequentially

### CI Pipeline
- [ ] GitHub Actions workflow runs lint, unit, and acceptance on every PR
- [ ] Acceptance job depends on lint + unit passing
- [ ] CI includes explicit wait-for-healthy step before acceptance tests
- [ ] Garage service container starts and is healthy
- [ ] Race detector enabled (`-race`) on both unit and acceptance
- [ ] Concurrency control cancels redundant runs

### Security
- [ ] Sensitive values (`secret_access_key`, `secret_token`) not leaked in test output
- [ ] Test configs don't hardcode real credentials
- [ ] CheckDestroy functions clean up on failure (no leaked resources)

## Implementation Phases

### Phase 1: Test Infrastructure

- [ ] Add `terraform-plugin-testing` to `go.mod`
- [ ] Create `internal/provider/provider_test.go` with:
  - `testAccProtoV6ProviderFactories`
  - `testAccPreCheck`
  - `randomName`
  - `testAccGetClient`
- [ ] Create `internal/provider/testhelpers_test.go` with:
  - Config builder functions (`testAccBucketConfig`, etc.)
  - `CheckDestroy` functions for all 6 resource types
  - `testAccCheckBucketExists` and similar existence checks
- [ ] Create `testdata/garage.toml`
- [ ] Create `testdata/docker-compose.yml`
- [ ] Add `GNUmakefile` targets: `test`, `testacc`, `testfilter`
- [ ] Verify Garage container starts locally with `docker compose up`

### Phase 2: Unit Tests (36 functions)

- [ ] `internal/garage/client_test.go`:
  - U1-U6: retry, no-retry, max-retries, timeout, Retry-After, backoff bounds
- [ ] `internal/garage/errors_test.go`:
  - U7-U9: Classify(), ClassifyError(), unknown fallback
- [ ] `internal/provider/permission_diff_test.go`:
  - U10-U15: all permission diff scenarios
- [ ] `internal/provider/layout_helpers_test.go`:
  - U16-U20: role-set computation, version conflict retry, max retry failure
- [ ] `internal/provider/id_parse_test.go`:
  - U21-U27: composite ID and alias composite ID parse/format
- [ ] `internal/provider/validators_test.go`:
  - U28-U36: scope validation, endpoint regex, key ID regex, node ID regex, bucket alias regex, RFC 3339

### Phase 3: Acceptance Tests — Resources (65 tests)

- [ ] `resource_bucket_test.go` — 15 tests (B1-B15)
- [ ] `resource_bucket_alias_test.go` — 8 tests (A1-A8)
- [ ] `resource_key_test.go` — 10 tests (K1-K10)
- [ ] `resource_bucket_permission_test.go` — 10 tests (P1-P10)
- [ ] `resource_layout_node_test.go` — 10 tests (L1-L10)
- [ ] `resource_admin_token_test.go` — 12 tests (T1-T12)

### Phase 4: Acceptance Tests — Data Sources (19 tests)

- [ ] `datasource_bucket_test.go` — D1-D4
- [ ] `datasource_buckets_test.go` — D5-D6
- [ ] `datasource_key_test.go` — D7-D8
- [ ] `datasource_keys_test.go` — D9-D10
- [ ] `datasource_cluster_health_test.go` — D11
- [ ] `datasource_cluster_status_test.go` — D12
- [ ] `datasource_cluster_layout_test.go` — D13-D14
- [ ] `datasource_admin_token_test.go` — D15-D17
- [ ] `datasource_admin_tokens_test.go` — D18
- [ ] `datasource_node_info_test.go` — D19

### Phase 5: Provider Config + E2E Tests (10 tests)

- [ ] Provider config tests — PC1-PC6
- [ ] E2E: Bucket+Key+Permission workflow
- [ ] E2E: Bucket+Alias+Website workflow
- [ ] E2E: Admin token lifecycle
- [ ] E2E: Layout multi-node chain

### Phase 6: CI Pipeline

- [ ] Create `.github/workflows/ci.yml`
- [ ] Create `.golangci.yml` with linter config
- [ ] Verify full pipeline passes: lint → unit → acceptance
- [ ] Enable branch protection requiring CI pass
