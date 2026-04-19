package garage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	t.Parallel()

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewGarageClient(srv.URL, "test-token",
		WithMaxRetries(3),
		WithRetryWait(0, 0),
	)
	if err != nil {
		t.Fatalf("NewGarageClient: %v", err)
	}

	status, err := client.DoWithRetry(context.Background(), func() (int, error) {
		resp, reqErr := http.Get(srv.URL)
		if reqErr != nil {
			return 0, reqErr
		}
		_ = resp.Body.Close()
		return resp.StatusCode, nil
	})

	if err != nil {
		t.Fatalf("DoWithRetry returned error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestDoWithRetry_NoRetryOnDeterministicErrors(t *testing.T) {
	t.Parallel()

	codes := []int{400, 401, 403, 404, 409}
	for _, code := range codes {
		code := code
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			t.Parallel()

			var attempts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(code)
			}))
			defer srv.Close()

			client, err := NewGarageClient(srv.URL, "test-token",
				WithMaxRetries(3),
				WithRetryWait(0, 0),
			)
			if err != nil {
				t.Fatalf("NewGarageClient: %v", err)
			}

			status, err := client.DoWithRetry(context.Background(), func() (int, error) {
				resp, reqErr := http.Get(srv.URL)
				if reqErr != nil {
					return 0, reqErr
				}
				_ = resp.Body.Close()
				return resp.StatusCode, nil
			})

			if err != nil {
				t.Fatalf("DoWithRetry returned error: %v", err)
			}
			if status != code {
				t.Fatalf("expected status %d, got %d", code, status)
			}
			if atomic.LoadInt32(&attempts) != 1 {
				t.Fatalf("expected 1 attempt (no retry), got %d", atomic.LoadInt32(&attempts))
			}
		})
	}
}

func TestDoWithRetry_MaxRetriesExhausted(t *testing.T) {
	t.Parallel()

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client, err := NewGarageClient(srv.URL, "test-token",
		WithMaxRetries(3),
		WithRetryWait(0, 0),
	)
	if err != nil {
		t.Fatalf("NewGarageClient: %v", err)
	}

	status, err := client.DoWithRetry(context.Background(), func() (int, error) {
		resp, reqErr := http.Get(srv.URL)
		if reqErr != nil {
			return 0, reqErr
		}
		_ = resp.Body.Close()
		return resp.StatusCode, nil
	})

	if err == nil {
		t.Fatal("expected error after max retries exhausted")
	}
	if status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", status)
	}
	// 1 initial + 3 retries = 4 attempts
	if atomic.LoadInt32(&attempts) != 4 {
		t.Fatalf("expected 4 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestDoWithRetry_ContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client, err := NewGarageClient(srv.URL, "test-token",
		WithMaxRetries(10),
		WithRetryWait(5, 10),
	)
	if err != nil {
		t.Fatalf("NewGarageClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.DoWithRetry(ctx, func() (int, error) {
		resp, reqErr := http.Get(srv.URL)
		if reqErr != nil {
			return 0, reqErr
		}
		_ = resp.Body.Close()
		return resp.StatusCode, nil
	})

	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		expected ErrorKind
	}{
		{"not found", 404, ErrorKindNotFound},
		{"conflict", 409, ErrorKindConflict},
		{"bad request", 400, ErrorKindValidation},
		{"unauthorized", 401, ErrorKindAuth},
		{"forbidden", 403, ErrorKindAuth},
		{"rate limited", 429, ErrorKindTransient},
		{"server error 500", 500, ErrorKindTransient},
		{"bad gateway 502", 502, ErrorKindTransient},
		{"service unavailable 503", 503, ErrorKindTransient},
		{"gateway timeout 504", 504, ErrorKindTransient},
		{"ok", 200, ErrorKindUnknown},
		{"created", 201, ErrorKindUnknown},
		{"no content", 204, ErrorKindUnknown},
		{"redirect", 301, ErrorKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Classify(tt.status)
			if got != tt.expected {
				t.Errorf("Classify(%d) = %d, want %d", tt.status, got, tt.expected)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected ErrorKind
	}{
		{"nil error", nil, ErrorKindUnknown},
		{"deadline exceeded", context.DeadlineExceeded, ErrorKindTransient},
		{"connection reset", fmt.Errorf("read tcp: connection reset by peer"), ErrorKindTransient},
		{"connection refused", fmt.Errorf("dial tcp: connection refused"), ErrorKindTransient},
		{"dns error", fmt.Errorf("dial tcp: lookup no-such-host: no such host"), ErrorKindTransient},
		{"io timeout", fmt.Errorf("net/http: i/o timeout"), ErrorKindTransient},
		{"eof", fmt.Errorf("unexpected EOF"), ErrorKindTransient},
		{"unknown error", fmt.Errorf("some random error"), ErrorKindTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyError(tt.err)
			if got != tt.expected {
				t.Errorf("ClassifyError(%v) = %d, want %d", tt.err, got, tt.expected)
			}
		})
	}
}

func TestRetryAfterDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		header  string
		wantDur time.Duration
		wantOK  bool
	}{
		{"valid seconds", "5", 5 * time.Second, true},
		{"zero", "0", 0, true},
		{"empty", "", 0, false},
		{"negative", "-1", 0, false},
		{"non-numeric", "abc", 0, false},
		{"date format", "Wed, 21 Oct 2015 07:28:00 GMT", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dur, ok := RetryAfterDuration(tt.header)
			if ok != tt.wantOK {
				t.Errorf("RetryAfterDuration(%q) ok = %v, want %v", tt.header, ok, tt.wantOK)
			}
			if dur != tt.wantDur {
				t.Errorf("RetryAfterDuration(%q) = %v, want %v", tt.header, dur, tt.wantDur)
			}
		})
	}
}

func TestNewGarageClient_AuthHeader(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewGarageClient(srv.URL, "my-secret-token",
		WithMaxRetries(0),
		WithRetryWait(0, 0),
	)
	if err != nil {
		t.Fatalf("NewGarageClient: %v", err)
	}

	_, err = client.Inner().HealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	expected := "Bearer my-secret-token"
	if gotAuth != expected {
		t.Errorf("Authorization header = %q, want %q", gotAuth, expected)
	}
}

func TestNewGarageClient_Defaults(t *testing.T) {
	t.Parallel()

	client, err := NewGarageClient("http://localhost:3903", "test-token")
	if err != nil {
		t.Fatalf("NewGarageClient: %v", err)
	}

	if client.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", client.maxRetries)
	}
	if client.minWait != 1*time.Second {
		t.Errorf("minWait = %v, want 1s", client.minWait)
	}
	if client.maxWait != 30*time.Second {
		t.Errorf("maxWait = %v, want 30s", client.maxWait)
	}
}

func TestNewGarageClient_WithOptions(t *testing.T) {
	t.Parallel()

	client, err := NewGarageClient("http://localhost:3903", "test-token",
		WithTimeout(60),
		WithMaxRetries(5),
		WithRetryWait(2, 20),
	)
	if err != nil {
		t.Fatalf("NewGarageClient: %v", err)
	}

	if client.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", client.maxRetries)
	}
	if client.minWait != 2*time.Second {
		t.Errorf("minWait = %v, want 2s", client.minWait)
	}
	if client.maxWait != 20*time.Second {
		t.Errorf("maxWait = %v, want 20s", client.maxWait)
	}
}

func TestBackoff_WithinBounds(t *testing.T) {
	t.Parallel()

	client := &GarageClient{
		minWait: 100 * time.Millisecond,
		maxWait: 5 * time.Second,
	}

	for attempt := 0; attempt < 10; attempt++ {
		d := client.backoff(attempt)
		if d < 0 {
			t.Errorf("attempt %d: backoff = %v, want >= 0", attempt, d)
		}
		maxExpected := client.minWait
		for i := 0; i < attempt; i++ {
			maxExpected *= 2
			if maxExpected > client.maxWait {
				maxExpected = client.maxWait
				break
			}
		}
		if d > maxExpected {
			t.Errorf("attempt %d: backoff = %v, want <= %v", attempt, d, maxExpected)
		}
	}
}

func TestIsRetryableStatus(t *testing.T) {
	t.Parallel()

	retryable := []int{429, 500, 502, 503, 504}
	for _, code := range retryable {
		if !isRetryableStatus(code) {
			t.Errorf("isRetryableStatus(%d) = false, want true", code)
		}
	}

	nonRetryable := []int{200, 201, 204, 301, 400, 401, 403, 404, 409}
	for _, code := range nonRetryable {
		if isRetryableStatus(code) {
			t.Errorf("isRetryableStatus(%d) = true, want false", code)
		}
	}
}
