package resilience

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDoEnforcesTimeout(t *testing.T) {
	err := Do(context.Background(), Policy{Timeout: time.Millisecond}, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Do() error = %v, want context deadline exceeded", err)
	}
}

func TestDependencyModeFor(t *testing.T) {
	tests := []struct {
		name string
		want DependencyMode
	}{
		{name: "postgres", want: DependencyStrong},
		{name: "redis", want: DependencyStrong},
		{name: "nats", want: DependencyStrong},
		{name: "kubernetes-api", want: DependencyStrong},
		{name: "object-store", want: DependencyWeak},
		{name: "vector-store", want: DependencyWeak},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DependencyModeFor(tt.name); got != tt.want {
				t.Fatalf("DependencyModeFor(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestRetryableClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "deadline",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "network",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			want: true,
		},
		{
			name: "status-503",
			err:  NewStatusError("Kubernetes API", "GET", "/version", 503, "unavailable"),
			want: true,
		},
		{
			name: "status-429",
			err:  NewStatusError("Kubernetes API", "GET", "/version", 429, "too many requests"),
			want: true,
		},
		{
			name: "status-400",
			err:  NewStatusError("Kubernetes API", "GET", "/version", 400, "bad request"),
		},
		{
			name: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Retryable(tt.err); got != tt.want {
				t.Fatalf("Retryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDoRetriesTransientThenSucceeds(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Policy{
		MaxAttempts: 3,
		BaseBackoff: time.Nanosecond,
		MaxBackoff:  time.Nanosecond,
	}, func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			return NewStatusError("Kubernetes API", "GET", "/version", 503, "unavailable")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestBreakerOpensAfterSustainedFailures(t *testing.T) {
	policy := Policy{
		MaxAttempts:    1,
		BreakerName:    t.Name(),
		FailureRatio:   0.5,
		MinRequests:    2,
		CooldownPeriod: time.Hour,
	}
	for i := 0; i < 2; i++ {
		err := Do(context.Background(), policy, func(ctx context.Context) error {
			return NewStatusError("Kubernetes API", "GET", "/version", 503, "unavailable")
		})
		if err == nil {
			t.Fatal("Do() error = nil, want transient failure")
		}
	}

	calls := 0
	err := Do(context.Background(), policy, func(ctx context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Do() error = %v, want ErrCircuitOpen", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want breaker to reject before invoking fn", calls)
	}
}
