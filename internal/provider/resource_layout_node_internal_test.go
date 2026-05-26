package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestBuildRoleChange ensures the request body sent to Garage's
// UpdateClusterLayout endpoint matches the NodeAssignedRole schema in all
// supported shapes. Regression test for
// https://github.com/vhco-pro/terraform-provider-garage/issues/1 — where nil
// tags marshaled to JSON null and Garage rejected the request with 400.
func TestBuildRoleChange(t *testing.T) {
	t.Parallel()

	const nodeID = "0000000000000000000000000000000000000000000000000000000000000001"

	tests := []struct {
		name        string
		zone        string
		capacity    types.Int64
		tags        []string
		remove      bool
		wantSubstrs []string
		notSubstrs  []string
	}{
		{
			name:        "omitted_tags_marshals_as_empty_array",
			zone:        "dc1",
			capacity:    types.Int64Value(1073741824),
			tags:        nil,
			wantSubstrs: []string{`"tags":[]`, `"zone":"dc1"`, `"capacity":1073741824`},
			notSubstrs:  []string{`"tags":null`},
		},
		{
			name:        "explicit_empty_tags_marshals_as_empty_array",
			zone:        "dc1",
			capacity:    types.Int64Value(1073741824),
			tags:        []string{},
			wantSubstrs: []string{`"tags":[]`},
			notSubstrs:  []string{`"tags":null`},
		},
		{
			name:        "populated_tags_preserved",
			zone:        "dc1",
			capacity:    types.Int64Value(1073741824),
			tags:        []string{"storage", "primary"},
			wantSubstrs: []string{`"tags":["storage","primary"]`},
		},
		{
			name:        "null_capacity_omitted_for_gateway",
			zone:        "dc1",
			capacity:    types.Int64Null(),
			tags:        []string{},
			wantSubstrs: []string{`"tags":[]`, `"zone":"dc1"`},
			notSubstrs:  []string{`"capacity"`},
		},
		{
			name:        "remove_shape",
			remove:      true,
			wantSubstrs: []string{`"remove":true`, `"id":"` + nodeID + `"`},
			notSubstrs:  []string{`"tags"`, `"zone"`, `"capacity"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc, err := buildRoleChange(nodeID, tt.zone, tt.capacity, tt.tags, tt.remove)
			if err != nil {
				t.Fatalf("buildRoleChange returned error: %v", err)
			}
			b, err := json.Marshal(rc)
			if err != nil {
				t.Fatalf("marshal role change: %v", err)
			}
			got := string(b)
			for _, want := range tt.wantSubstrs {
				if !contains(got, want) {
					t.Errorf("payload %s missing %q", got, want)
				}
			}
			for _, bad := range tt.notSubstrs {
				if contains(got, bad) {
					t.Errorf("payload %s must not contain %q", got, bad)
				}
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
