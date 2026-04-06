package provider

import (
	"testing"
)

func TestRegexpHTTPEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		match bool
	}{
		{"http localhost", "http://localhost:3903", true},
		{"https domain", "https://garage.example.com", true},
		{"https with path", "https://garage.example.com/admin", true},
		{"ftp", "ftp://example.com", false},
		{"no scheme", "localhost:3903", false},
		{"empty", "", false},
		{"just http", "http://", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := regexpHTTPEndpoint.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("regexpHTTPEndpoint.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestRegexpBucketAlias(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		match bool
	}{
		{"simple", "my-bucket", true},
		{"with dots", "my.bucket.name", true},
		{"min length 3", "abc", true},
		{"max length 63", "a234567890123456789012345678901234567890123456789012345678901bc", true},
		{"too short 2", "ab", false},
		{"too long 64", "a2345678901234567890123456789012345678901234567890123456789012bc", false},
		{"uppercase", "My-Bucket", false},
		{"starts with dash", "-bucket", false},
		{"ends with dash", "bucket-", false},
		{"starts with dot", ".bucket", false},
		{"numeric only", "123", true},
		{"all valid chars", "a0-b1.c2", true},
		{"empty", "", false},
		{"single char", "a", false},
		{"underscore", "my_bucket", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := regexpBucketAlias.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("regexpBucketAlias.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestRegexpKeyID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		match bool
	}{
		{"valid", "GK0123456789abcdef01234567", true},
		{"all hex lower", "GKabcdefabcdefabcdefabcdef", true},
		{"all zero", "GK000000000000000000000000", true},
		{"too short", "GK0123456789abcdef0123456", false},
		{"too long", "GK0123456789abcdef012345678", false},
		{"wrong prefix", "XX0123456789abcdef01234567", false},
		{"no prefix", "0123456789abcdef0123456789", false},
		{"uppercase hex", "GK0123456789ABCDEF01234567", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := regexpKeyID.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("regexpKeyID.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestRegexpNodeID(t *testing.T) {
	t.Parallel()
	validNodeID := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		name  string
		input string
		match bool
	}{
		{"valid 64 hex", validNodeID, true},
		{"too short 63", validNodeID[:63], false},
		{"too long 65", validNodeID + "0", false},
		{"uppercase", "0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef", false},
		{"non-hex", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := regexpNodeID.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("regexpNodeID.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}
