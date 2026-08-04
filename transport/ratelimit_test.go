package transport

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestRateLimiter_Wait_RPS(t *testing.T) {
	rl := NewRateLimiter(WithRPS(100)) // 100 rps = 10ms interval

	start := time.Now()
	rl.Wait()
	rl.Wait()
	elapsed := time.Since(start)

	// Second wait should have been delayed by ~10ms
	if elapsed < 5*time.Millisecond {
		t.Errorf("expected at least 5ms delay, got %v", elapsed)
	}
}

func TestRateLimiter_Wait_MinDelay(t *testing.T) {
	rl := NewRateLimiter(WithMinDelay(20 * time.Millisecond))

	start := time.Now()
	rl.Wait()
	rl.Wait()
	elapsed := time.Since(start)

	if elapsed < 15*time.Millisecond {
		t.Errorf("expected at least 15ms delay, got %v", elapsed)
	}
}

func TestRateLimiter_UpdateFromResponse(t *testing.T) {
	rl := NewRateLimiter()

	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "42")
	h.Set("X-RateLimit-Reset", "1700000000")
	resp := &http.Response{Header: h}
	rl.UpdateFromResponse(resp)

	if rl.Remaining() != 42 {
		t.Errorf("expected remaining 42, got %d", rl.Remaining())
	}

	expectedReset := time.Unix(1700000000, 0)
	if !rl.ResetAt().Equal(expectedReset) {
		t.Errorf("expected reset at %v, got %v", expectedReset, rl.ResetAt())
	}
}

func TestRateLimiter_UpdateFromResponse_Nil(t *testing.T) {
	rl := NewRateLimiter()
	rl.UpdateFromResponse(nil) // should not panic
}

func TestRateLimiter_UpdateFromResponse_MissingHeaders(t *testing.T) {
	rl := NewRateLimiter()

	resp := &http.Response{Header: http.Header{}}
	rl.UpdateFromResponse(resp)

	if rl.Remaining() != 0 {
		t.Errorf("expected remaining 0, got %d", rl.Remaining())
	}
}

func TestRateLimiter_UpdateFromResponse_InvalidValues(t *testing.T) {
	rl := NewRateLimiter()

	resp := &http.Response{
		Header: http.Header{
			"X-RateLimit-Remaining": []string{"not-a-number"},
			"X-RateLimit-Reset":     []string{"not-a-number"},
		},
	}
	rl.UpdateFromResponse(resp) // should not panic

	if rl.Remaining() != 0 {
		t.Errorf("expected remaining 0, got %d", rl.Remaining())
	}
}

func TestRateLimiter_AdaptiveThrottle(t *testing.T) {
	rl := NewRateLimiter(WithThrottleThreshold(5))

	// Simulate low remaining quota with reset far in the future
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "3")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(10*time.Second).Unix(), 10))
	resp := &http.Response{Header: h}
	rl.UpdateFromResponse(resp)

	start := time.Now()
	rl.Wait()
	elapsed := time.Since(start)

	// Should have waited some time to spread requests
	if elapsed < 500*time.Millisecond {
		t.Errorf("expected adaptive throttle delay, got %v", elapsed)
	}
}

func TestRateLimiter_NoLimit(t *testing.T) {
	rl := NewRateLimiter() // no RPS, no min delay

	start := time.Now()
	rl.Wait()
	rl.Wait()
	elapsed := time.Since(start)

	// Should return immediately
	if elapsed > 5*time.Millisecond {
		t.Errorf("expected near-instant return, got %v", elapsed)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(WithRPS(1000))

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			rl.Wait()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// Should not panic or deadlock
}
