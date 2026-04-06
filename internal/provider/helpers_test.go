package provider

import (
	"testing"
)

func TestComputePermissionDiff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		current permissionFlags
		desired permissionFlags
		allow   permissionFlags
		deny    permissionFlags
	}{
		{
			name:    "grant read from empty",
			current: permissionFlags{},
			desired: permissionFlags{Read: true},
			allow:   permissionFlags{Read: true},
			deny:    permissionFlags{},
		},
		{
			name:    "revoke write keep read",
			current: permissionFlags{Read: true, Write: true},
			desired: permissionFlags{Read: true},
			allow:   permissionFlags{},
			deny:    permissionFlags{Write: true},
		},
		{
			name:    "no-op when unchanged",
			current: permissionFlags{Read: true},
			desired: permissionFlags{Read: true},
			allow:   permissionFlags{},
			deny:    permissionFlags{},
		},
		{
			name:    "full grant from empty",
			current: permissionFlags{},
			desired: permissionFlags{Read: true, Write: true, Owner: true},
			allow:   permissionFlags{Read: true, Write: true, Owner: true},
			deny:    permissionFlags{},
		},
		{
			name:    "full revoke to empty",
			current: permissionFlags{Read: true, Write: true, Owner: true},
			desired: permissionFlags{},
			allow:   permissionFlags{},
			deny:    permissionFlags{Read: true, Write: true, Owner: true},
		},
		{
			name:    "mixed grant and revoke",
			current: permissionFlags{Read: true, Owner: true},
			desired: permissionFlags{Read: true, Write: true},
			allow:   permissionFlags{Write: true},
			deny:    permissionFlags{Owner: true},
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

func TestParsePermissionID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		id          string
		wantBucket  string
		wantKey     string
		expectError bool
	}{
		{
			name:       "valid",
			id:         "abc123/GK0000000000000000000000000",
			wantBucket: "abc123",
			wantKey:    "GK0000000000000000000000000",
		},
		{
			name:        "missing separator",
			id:          "no-separator",
			expectError: true,
		},
		{
			name:        "empty bucket",
			id:          "/GK0000000000000000000000000",
			expectError: true,
		},
		{
			name:        "empty key",
			id:          "abc123/",
			expectError: true,
		},
		{
			name:       "extra separators preserved",
			id:         "abc123/key/extra",
			wantBucket: "abc123",
			wantKey:    "key/extra",
		},
		{
			name:        "empty string",
			id:          "",
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bucket, key, err := parsePermissionID(tt.id)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.id)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.id, err)
				return
			}
			if bucket != tt.wantBucket {
				t.Errorf("bucket: got %q, want %q", bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("key: got %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestFormatPermissionID(t *testing.T) {
	t.Parallel()
	got := formatPermissionID("bucket-abc", "GK1234")
	want := "bucket-abc/GK1234"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseAliasID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		id          string
		want        aliasID
		expectError bool
	}{
		{
			name: "global alias",
			id:   "bucket123:global:my-alias",
			want: aliasID{BucketID: "bucket123", AliasType: "global", Name: "my-alias"},
		},
		{
			name: "local alias",
			id:   "bucket123:local:GK0000000000000000000000000:my-alias",
			want: aliasID{BucketID: "bucket123", AliasType: "local", AccessKeyID: "GK0000000000000000000000000", Name: "my-alias"},
		},
		{
			name:        "invalid type in 3-part",
			id:          "bucket:invalid:name",
			expectError: true,
		},
		{
			name:        "invalid type in 4-part",
			id:          "bucket:invalid:key:name",
			expectError: true,
		},
		{
			name:        "too few parts",
			id:          "bucket:global",
			expectError: true,
		},
		{
			name:        "too many parts",
			id:          "a:b:c:d:e",
			expectError: true,
		},
		{
			name:        "empty bucket in global",
			id:          ":global:name",
			expectError: true,
		},
		{
			name:        "empty name in global",
			id:          "bucket:global:",
			expectError: true,
		},
		{
			name:        "empty key in local",
			id:          "bucket:local::name",
			expectError: true,
		},
		{
			name:        "empty string",
			id:          "",
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseAliasID(tt.id)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.id)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.id, err)
				return
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFormatAliasID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a    aliasID
		want string
	}{
		{
			name: "global",
			a:    aliasID{BucketID: "b1", AliasType: "global", Name: "alias1"},
			want: "b1:global:alias1",
		},
		{
			name: "local",
			a:    aliasID{BucketID: "b1", AliasType: "local", AccessKeyID: "k1", Name: "alias1"},
			want: "b1:local:k1:alias1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatAliasID(tt.a)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAliasIDRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []aliasID{
		{BucketID: "bucket-abc", AliasType: "global", Name: "my-alias"},
		{BucketID: "bucket-def", AliasType: "local", AccessKeyID: "GK1234", Name: "local-alias"},
	}
	for _, original := range tests {
		t.Run(original.AliasType, func(t *testing.T) {
			t.Parallel()
			formatted := formatAliasID(original)
			parsed, err := parseAliasID(formatted)
			if err != nil {
				t.Fatalf("round-trip parse error: %v", err)
			}
			if parsed != original {
				t.Errorf("round-trip mismatch: got %+v, want %+v", parsed, original)
			}
		})
	}
}
