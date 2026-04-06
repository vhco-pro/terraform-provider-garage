package provider

import (
	"fmt"
	"strings"
)

// permissionDiff computes which permission flags need to be allowed/denied
// given a transition from current to desired state.
type permissionFlags struct {
	Read  bool
	Write bool
	Owner bool
}

func computePermissionDiff(current, desired permissionFlags) (allow, deny permissionFlags) {
	if desired.Read && !current.Read {
		allow.Read = true
	} else if !desired.Read && current.Read {
		deny.Read = true
	}
	if desired.Write && !current.Write {
		allow.Write = true
	} else if !desired.Write && current.Write {
		deny.Write = true
	}
	if desired.Owner && !current.Owner {
		allow.Owner = true
	} else if !desired.Owner && current.Owner {
		deny.Owner = true
	}
	return
}

// parsePermissionID parses a composite ID of the form "bucket_id/access_key_id".
func parsePermissionID(id string) (bucketID, accessKeyID string, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected format: {bucket_id}/{access_key_id}, got: %q", id)
	}
	return parts[0], parts[1], nil
}

// formatPermissionID formats a composite permission ID.
func formatPermissionID(bucketID, accessKeyID string) string {
	return bucketID + "/" + accessKeyID
}

// aliasID represents a parsed bucket alias composite ID.
type aliasID struct {
	BucketID    string
	AliasType   string // "global" or "local"
	Name        string
	AccessKeyID string // only set for local aliases
}

// parseAliasID parses a composite alias ID.
// Global format: {bucket_id}:global:{name}
// Local format:  {bucket_id}:local:{key_id}:{name}
func parseAliasID(id string) (aliasID, error) {
	parts := strings.Split(id, ":")
	switch len(parts) {
	case 3:
		if parts[1] != "global" {
			return aliasID{}, fmt.Errorf("expected format: {bucket_id}:global:{name} or {bucket_id}:local:{key_id}:{name}, got: %q", id)
		}
		if parts[0] == "" || parts[2] == "" {
			return aliasID{}, fmt.Errorf("empty parts in alias ID: %q", id)
		}
		return aliasID{BucketID: parts[0], AliasType: "global", Name: parts[2]}, nil
	case 4:
		if parts[1] != "local" {
			return aliasID{}, fmt.Errorf("expected format: {bucket_id}:global:{name} or {bucket_id}:local:{key_id}:{name}, got: %q", id)
		}
		if parts[0] == "" || parts[2] == "" || parts[3] == "" {
			return aliasID{}, fmt.Errorf("empty parts in alias ID: %q", id)
		}
		return aliasID{BucketID: parts[0], AliasType: "local", AccessKeyID: parts[2], Name: parts[3]}, nil
	default:
		return aliasID{}, fmt.Errorf("expected format: {bucket_id}:global:{name} or {bucket_id}:local:{key_id}:{name}, got: %q", id)
	}
}

// formatAliasID formats a composite alias ID from its parts.
func formatAliasID(a aliasID) string {
	if a.AliasType == "global" {
		return a.BucketID + ":global:" + a.Name
	}
	return a.BucketID + ":local:" + a.AccessKeyID + ":" + a.Name
}
