package provider

import "regexp"

var (
	regexpHTTPEndpoint = regexp.MustCompile(`^https?://`)
	regexpBucketAlias  = regexp.MustCompile(`^[a-z0-9][a-z0-9.\-]{1,61}[a-z0-9]$`)
	regexpKeyID        = regexp.MustCompile(`^GK[0-9a-f]{24}$`)
	regexpNodeID       = regexp.MustCompile(`^[0-9a-f]{64}$`)
)
