package garage

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"
)

// GarageClient wraps the generated oapi-codegen client with operational concerns.
type GarageClient struct {
	inner      *ClientWithResponses
	maxRetries int
	minWait    time.Duration
	maxWait    time.Duration
}

// GarageClientOption configures optional GarageClient behavior.
type GarageClientOption func(*garageClientConfig)

type garageClientConfig struct {
	timeout    time.Duration
	maxRetries int
	minWait    time.Duration
	maxWait    time.Duration
}

// WithTimeout sets the HTTP client timeout in seconds.
func WithTimeout(seconds int64) GarageClientOption {
	return func(c *garageClientConfig) {
		c.timeout = time.Duration(seconds) * time.Second
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) GarageClientOption {
	return func(c *garageClientConfig) {
		c.maxRetries = n
	}
}

// WithRetryWait sets the backoff bounds in seconds.
func WithRetryWait(minSeconds, maxSeconds int64) GarageClientOption {
	return func(c *garageClientConfig) {
		c.minWait = time.Duration(minSeconds) * time.Second
		c.maxWait = time.Duration(maxSeconds) * time.Second
	}
}

// NewGarageClient creates a GarageClient with the given endpoint, token, and options.
func NewGarageClient(endpoint string, token string, opts ...GarageClientOption) (*GarageClient, error) {
	cfg := &garageClientConfig{
		timeout:    30 * time.Second,
		maxRetries: 3,
		minWait:    1 * time.Second,
		maxWait:    30 * time.Second,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	httpClient := &http.Client{
		Timeout: cfg.timeout,
	}

	authEditor := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	inner, err := NewClientWithResponses(endpoint,
		WithHTTPClient(httpClient),
		WithRequestEditorFn(authEditor),
	)
	if err != nil {
		return nil, fmt.Errorf("creating garage API client: %w", err)
	}

	return &GarageClient{
		inner:      inner,
		maxRetries: cfg.maxRetries,
		minWait:    cfg.minWait,
		maxWait:    cfg.maxWait,
	}, nil
}

// Inner returns the underlying generated client for direct API access.
func (c *GarageClient) Inner() *ClientWithResponses {
	return c.inner
}

// ErrorKind classifies API errors for resource-level decision making.
type ErrorKind int

const (
	ErrorKindTransient  ErrorKind = iota // 429, 5xx, network — retryable
	ErrorKindNotFound                    // 404 — resource doesn't exist
	ErrorKindConflict                    // 409 — concurrent modification
	ErrorKindValidation                  // 400 — bad request
	ErrorKindAuth                        // 401, 403 — authentication/authorization
	ErrorKindUnknown                     // anything else
)

// Classify returns the ErrorKind for an HTTP status code.
func Classify(statusCode int) ErrorKind {
	switch {
	case statusCode == 404:
		return ErrorKindNotFound
	case statusCode == 409:
		return ErrorKindConflict
	case statusCode == 400:
		return ErrorKindValidation
	case statusCode == 401 || statusCode == 403:
		return ErrorKindAuth
	case statusCode == 429:
		return ErrorKindTransient
	case statusCode >= 500 && statusCode < 600:
		return ErrorKindTransient
	default:
		return ErrorKindUnknown
	}
}

// ClassifyError returns the ErrorKind for a Go error (network errors, DNS, timeouts).
// Unknown network errors are classified as transient since they typically indicate
// temporary connectivity issues that may resolve on retry.
func ClassifyError(err error) ErrorKind {
	if err == nil {
		return ErrorKindUnknown
	}

	if err == context.DeadlineExceeded {
		return ErrorKindTransient
	}

	errStr := err.Error()
	for _, pattern := range []string{
		"connection reset",
		"connection refused",
		"no such host",
		"i/o timeout",
		"EOF",
	} {
		if stringContains(errStr, pattern) {
			return ErrorKindTransient
		}
	}

	// Network errors not matching known patterns are still likely transient.
	return ErrorKindTransient
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// DoWithRetry executes an HTTP request function with retry logic.
func (c *GarageClient) DoWithRetry(ctx context.Context, fn func() (int, error)) (int, error) {
	var lastStatus int
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		lastStatus, lastErr = fn()

		if lastErr != nil {
			kind := ClassifyError(lastErr)
			if kind != ErrorKindTransient {
				return lastStatus, lastErr
			}
		} else if !isRetryableStatus(lastStatus) {
			return lastStatus, nil
		}

		if attempt < c.maxRetries {
			wait := c.backoff(attempt)
			select {
			case <-ctx.Done():
				return lastStatus, ctx.Err()
			case <-time.After(wait):
			}
		}
	}

	if lastErr != nil {
		return lastStatus, fmt.Errorf("max retries (%d) exhausted: %w", c.maxRetries, lastErr)
	}
	return lastStatus, fmt.Errorf("max retries (%d) exhausted, last status: %d", c.maxRetries, lastStatus)
}

func isRetryableStatus(status int) bool {
	switch status {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

func (c *GarageClient) backoff(attempt int) time.Duration {
	exp := c.minWait
	for i := 0; i < attempt; i++ {
		exp *= 2
		if exp > c.maxWait {
			exp = c.maxWait
			break
		}
	}
	if exp <= 0 {
		return c.minWait
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(exp)))
	if err != nil {
		return exp
	}
	return time.Duration(n.Int64())
}

// RetryAfterDuration parses a Retry-After header value (seconds) into a Duration.
func RetryAfterDuration(header string) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}
	seconds, err := strconv.Atoi(header)
	if err != nil {
		return 0, false
	}
	if seconds < 0 {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}
