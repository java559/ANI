package resilience

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Policy struct {
	Timeout        time.Duration
	MaxAttempts    int
	BaseBackoff    time.Duration
	MaxBackoff     time.Duration
	BreakerName    string
	FailureRatio   float64
	MinRequests    uint32
	CooldownPeriod time.Duration
}

var ErrCircuitOpen = errors.New("circuit open")

type StatusError struct {
	System     string
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func NewStatusError(system string, method string, path string, statusCode int, body string) error {
	return &StatusError{
		System:     strings.TrimSpace(system),
		Method:     strings.TrimSpace(method),
		Path:       strings.TrimSpace(path),
		StatusCode: statusCode,
		Body:       strings.TrimSpace(body),
	}
}

func (e *StatusError) Error() string {
	target := strings.TrimSpace(e.Method + " " + e.Path)
	if target == "" {
		target = "request"
	}
	message := fmt.Sprintf("%s %s returned HTTP %d", e.System, target, e.StatusCode)
	if e.Body != "" {
		message += ": " + e.Body
	}
	return strings.TrimSpace(message)
}

func Retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == 429 || statusErr.StatusCode >= 500
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		type temporary interface {
			Temporary() bool
		}
		if temp, ok := netErr.(temporary); ok && temp.Temporary() {
			return true
		}
	}
	return false
}

func Do(ctx context.Context, policy Policy, fn func(context.Context) error) error {
	breaker := breakerFor(policy)
	if breaker != nil && !breaker.allow(time.Now(), policy.CooldownPeriod) {
		return ErrCircuitOpen
	}

	attempts := policy.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if breaker != nil {
				breaker.record(false, policy, time.Now())
			}
			return err
		}

		err := doOnce(ctx, policy.Timeout, fn)
		if err == nil {
			if breaker != nil {
				breaker.record(true, policy, time.Now())
			}
			return nil
		}
		lastErr = err
		if !Retryable(err) || attempt == attempts {
			if breaker != nil {
				breaker.record(false, policy, time.Now())
			}
			return err
		}
		if err := sleepBackoff(ctx, policy, attempt); err != nil {
			if breaker != nil {
				breaker.record(false, policy, time.Now())
			}
			return err
		}
	}
	if breaker != nil {
		breaker.record(false, policy, time.Now())
	}
	return lastErr
}

func doOnce(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	if timeout <= 0 {
		return fn(ctx)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(callCtx)
}

func sleepBackoff(ctx context.Context, policy Policy, attempt int) error {
	delay := policy.BaseBackoff
	if delay <= 0 {
		return nil
	}
	for i := 1; i < attempt; i++ {
		delay *= 2
		if policy.MaxBackoff > 0 && delay >= policy.MaxBackoff {
			delay = policy.MaxBackoff
			break
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var circuitBreakers sync.Map

type circuitBreaker struct {
	mu       sync.Mutex
	requests uint32
	failures uint32
	openedAt time.Time
	halfOpen bool
}

func breakerFor(policy Policy) *circuitBreaker {
	if strings.TrimSpace(policy.BreakerName) == "" || policy.MinRequests == 0 || policy.FailureRatio <= 0 {
		return nil
	}
	value, _ := circuitBreakers.LoadOrStore(policy.BreakerName, &circuitBreaker{})
	return value.(*circuitBreaker)
}

func (b *circuitBreaker) allow(now time.Time, cooldown time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openedAt.IsZero() {
		return true
	}
	if cooldown <= 0 || now.Sub(b.openedAt) >= cooldown {
		b.halfOpen = true
		return true
	}
	return false
}

func (b *circuitBreaker) record(success bool, policy Policy, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.halfOpen {
		if success {
			b.requests = 0
			b.failures = 0
			b.openedAt = time.Time{}
			b.halfOpen = false
			return
		}
		b.openedAt = now
		b.halfOpen = false
		return
	}
	if !b.openedAt.IsZero() {
		return
	}
	b.requests++
	if !success {
		b.failures++
	}
	if b.requests >= policy.MinRequests && float64(b.failures)/float64(b.requests) >= policy.FailureRatio {
		b.openedAt = now
	}
}
