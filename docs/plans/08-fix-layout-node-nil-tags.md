---
status: in-progress
status_description: "Root cause identified. TDD: failing unit test, then refactor + fix, then acceptance coverage. Tracks GitHub issue #1."
description: "Fix garage_layout_node 400 Bad Request when the tags attribute is omitted from HCL"
author: "garage-operator team"
goal: "Make garage_layout_node usable without explicitly declaring the optional tags attribute, matching the Garage Admin API contract"
priority: high
created: 2026-05-26
---

# Plan: Fix `garage_layout_node` nil `tags` regression (#1)

A user reported (GitHub issue [vhco-pro/terraform-provider-garage#1](https://github.com/vhco-pro/terraform-provider-garage/issues/1)) that creating a `garage_layout_node` from scratch fails with:

```
Error: Error creating layout node
  staging layout change: unexpected status 400 Bad Request
```

This plan documents the root cause, the fix, and the test coverage that prevents regression.

## Context

The provider's `garage_layout_node` resource performs a two-phase layout change (`UpdateClusterLayout` → `ApplyClusterLayout`). The failure surfaces at the **stage** step, not the apply step — the Garage Admin API rejects the request body with `400 Bad Request` before any layout transition is attempted.

The Garage Admin API v2.2.0 `NodeAssignedRole` schema (vendored at `internal/garage/openapi/spec.json`) declares:

```json
"NodeAssignedRole": {
  "type": "object",
  "required": ["zone", "tags"],
  "properties": {
    "tags": { "type": "array", "items": { "type": "string" } },
    "zone": { "type": "string" },
    "capacity": { "type": ["integer", "null"] }
  }
}
```

`tags` is **required** and must be a JSON **array**. A `null` value violates the schema.

In `internal/provider/resource_layout_node.go` (`Create` and `Update`), the local `tags` variable is left as a nil slice when the HCL omits the optional `tags` attribute:

```go
var tags []string
if !plan.Tags.IsNull() {
    resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
}
```

That nil slice is passed straight into the generated `garage.NodeRoleChangeRequest1` whose field is declared without `omitempty`:

```go
Tags []string `json:"tags"`
```

A nil Go slice marshals to JSON `null`, producing a body like:

```json
{"id":"…","zone":"example","capacity":53687091200,"tags":null}
```

which Garage rejects with `400 Bad Request`.

### Why this was not caught

`TestAccLayoutNode_lifecycle` (in `internal/provider/resource_layout_node_test.go`) only exercises configurations where `tags` is explicitly set — either to a non-empty array (`["storage"]`, `["storage","primary"]`) or to an empty array literal (`[]`). The HCL omission case (`tags` attribute absent entirely) is untested.

Related plan: [Plan 04 — Cluster Layout](04-cluster-layout.md).

## Scope

### In Scope

- Fix `garage_layout_node` so omitting `tags` in HCL succeeds (sends `"tags": []`).
- Add unit-test coverage for the role-change construction.
- Add acceptance-test coverage for the omitted-tags configuration.
- Documentation: addendum on Plan 04, this plan, link from issue #1.

### Out of Scope

- A new `garage_cluster_layout` aggregate resource (separate ergonomic concern raised in the same issue — tracked separately).
- A multi-node example for `examples/` (tracked separately).
- Regenerating the OpenAPI client to add `omitempty` (would change wire semantics for *all* required arrays; fixing at the caller is safer).

## Design

### Root cause summary

| Layer | Behavior |
|---|---|
| HCL: `tags` omitted | `plan.Tags.IsNull()` → `true` |
| Resource code | `var tags []string` stays nil; passes nil into `NodeRoleChangeRequest1{Tags: tags}` |
| Generated client | `Tags []string \`json:"tags"\`` (no `omitempty`) |
| `encoding/json` | nil slice → `null` |
| Garage API | `tags` is required `array` → `400 Bad Request` |

### Fix

Extract role-change construction from `stageAndApply` into a pure helper so it is unit-testable, and normalize nil tags to an empty slice inside the helper:

```go
func buildRoleChange(nodeID, zone string, capacity types.Int64, tags []string, remove bool) (garage.NodeRoleChangeRequest, error) {
    var rc garage.NodeRoleChangeRequest
    if remove {
        return rc, rc.FromNodeRoleChangeRequest0(garage.NodeRoleChangeRequest0{Id: nodeID, Remove: true})
    }
    if tags == nil {
        tags = []string{} // Garage NodeAssignedRole.tags is required array; nil → JSON null → 400
    }
    req1 := garage.NodeRoleChangeRequest1{Id: nodeID, Zone: zone, Tags: tags}
    if !capacity.IsNull() {
        v := capacity.ValueInt64()
        req1.Capacity = &v
    }
    return rc, rc.FromNodeRoleChangeRequest1(req1)
}
```

`stageAndApply` calls `buildRoleChange` instead of inlining the construction. No behavior changes when `tags` is already non-nil (empty or populated).

### Why fix at the caller (not the generated client)

The generated client is faithful to the OpenAPI spec — `tags` is required, so `omitempty` would be wrong at the schema level. The provider is the layer that knows "optional in HCL ⇒ empty array on the wire", so it owns the normalization.

## Acceptance Criteria

- [ ] Unit test: `buildRoleChange` with `nil` tags produces JSON whose `tags` field is `[]`, not `null`.
- [ ] Unit test: `buildRoleChange` with `[]string{}` tags produces JSON whose `tags` field is `[]`.
- [ ] Unit test: `buildRoleChange` with `["a","b"]` tags produces JSON whose `tags` field is `["a","b"]`.
- [ ] Unit test: `buildRoleChange` with `remove=true` produces JSON with `remove: true` and no `tags` field.
- [ ] Unit test: `buildRoleChange` with null `capacity` produces JSON without a `capacity` field.
- [ ] Acceptance test: `garage_layout_node` with HCL that omits the `tags` attribute creates successfully and reports `tags.# = 0`.
- [ ] Existing `TestAccLayoutNode_lifecycle` continues to pass unchanged.
- [ ] Lint clean (`golangci-lint run`).
- [ ] `make build` succeeds.

## Implementation Phases

### Phase 1: Failing test (TDD)

- [ ] Add `TestBuildRoleChange_omittedTags` to `internal/provider/resource_layout_node_test.go` (or new file). Confirm it fails against current code (helper does not exist yet, or after extraction it sends `null`).

### Phase 2: Refactor + fix

- [ ] Extract `buildRoleChange` from `stageAndApply`.
- [ ] Normalize nil tags to `[]string{}` inside the helper.
- [ ] Update `stageAndApply` to call the helper.
- [ ] Confirm unit tests pass.

### Phase 3: Acceptance coverage

- [ ] Add an acceptance test (or new step on `TestAccLayoutNode_lifecycle`) covering HCL that omits `tags`.
- [ ] Confirm test passes against a real Garage when `TF_ACC=1`.

### Phase 4: Documentation + release prep

- [ ] Update Plan 04 with a brief "Known Issues / Addenda" entry pointing to this plan and issue #1.
- [ ] Commit with conventional commit referencing issue #1 (`fix(layout_node): ... (#1)`).
- [ ] On merge to `main`, GoReleaser cuts a patch release (`v0.1.x`) per existing release workflow.

## Test Plan

| Acceptance Criterion | Test ID | Test Type | Location |
|---|---|---|---|
| nil tags → `"tags":[]` | U-LN-1 | Unit | `internal/provider/resource_layout_node_test.go` |
| empty tags → `"tags":[]` | U-LN-2 | Unit | `internal/provider/resource_layout_node_test.go` |
| populated tags preserved | U-LN-3 | Unit | `internal/provider/resource_layout_node_test.go` |
| remove=true shape | U-LN-4 | Unit | `internal/provider/resource_layout_node_test.go` |
| null capacity omitted | U-LN-5 | Unit | `internal/provider/resource_layout_node_test.go` |
| HCL without `tags` creates node | A-LN-1 | Acceptance | `internal/provider/resource_layout_node_test.go` |

## Open Questions

None. Fix is well-scoped and behavior-preserving for all existing usage.
