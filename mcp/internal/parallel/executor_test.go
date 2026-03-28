package parallel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelExecutor_BasicExecution(t *testing.T) {
	pe := &ParallelExecutor{Concurrency: 2}

	jobs := []ParallelJob{
		{PerspectiveID: "p1", Fn: func(ctx context.Context) (string, error) {
			return "/out/p1", nil
		}},
		{PerspectiveID: "p2", Fn: func(ctx context.Context) (string, error) {
			return "/out/p2", nil
		}},
		{PerspectiveID: "p3", Fn: func(ctx context.Context) (string, error) {
			return "/out/p3", nil
		}},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 3 {
		t.Errorf("Succeeded = %d, want 3", pr.Succeeded)
	}
	if pr.Failed != 0 {
		t.Errorf("Failed = %d, want 0", pr.Failed)
	}
	if len(pr.Results) != 3 {
		t.Fatalf("Results len = %d, want 3", len(pr.Results))
	}

	// Results should be indexed in order
	for i, pid := range []string{"p1", "p2", "p3"} {
		if pr.Results[i].PerspectiveID != pid {
			t.Errorf("Results[%d].PerspectiveID = %q, want %q", i, pr.Results[i].PerspectiveID, pid)
		}
		if pr.Results[i].OutputPath != fmt.Sprintf("/out/%s", pid) {
			t.Errorf("Results[%d].OutputPath = %q, want /out/%s", i, pr.Results[i].OutputPath, pid)
		}
	}
}

func TestParallelExecutor_ConcurrencyLimit(t *testing.T) {
	var active int64
	var maxActive int64

	pe := &ParallelExecutor{Concurrency: 2}

	jobs := make([]ParallelJob, 6)
	for i := range jobs {
		pid := fmt.Sprintf("p%d", i)
		jobs[i] = ParallelJob{
			PerspectiveID: pid,
			Fn: func(ctx context.Context) (string, error) {
				cur := atomic.AddInt64(&active, 1)
				// Track max concurrent
				for {
					old := atomic.LoadInt64(&maxActive)
					if cur <= old || atomic.CompareAndSwapInt64(&maxActive, old, cur) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt64(&active, -1)
				return "/out/" + pid, nil
			},
		}
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 6 {
		t.Errorf("Succeeded = %d, want 6", pr.Succeeded)
	}

	observed := atomic.LoadInt64(&maxActive)
	if observed > 2 {
		t.Errorf("max concurrent = %d, want <= 2 (concurrency limit)", observed)
	}
	if observed < 1 {
		t.Error("max concurrent should be at least 1")
	}
}

func TestParallelExecutor_RetryOnFailure(t *testing.T) {
	var attempts int64

	pe := &ParallelExecutor{
		Concurrency: 1,
		RetryLimit:  2,
	}

	jobs := []ParallelJob{
		{
			PerspectiveID: "retry-test",
			Fn: func(ctx context.Context) (string, error) {
				a := atomic.AddInt64(&attempts, 1)
				if a == 1 {
					return "", fmt.Errorf("transient error")
				}
				return "/out/retry-test", nil
			},
		},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1 (should succeed on retry)", pr.Succeeded)
	}
	if atomic.LoadInt64(&attempts) != 2 {
		t.Errorf("attempts = %d, want 2", atomic.LoadInt64(&attempts))
	}
}

func TestParallelExecutor_RetryExhausted(t *testing.T) {
	pe := &ParallelExecutor{
		Concurrency: 1,
		RetryLimit:  2,
	}

	jobs := []ParallelJob{
		{
			PerspectiveID: "always-fail",
			Fn: func(ctx context.Context) (string, error) {
				return "", fmt.Errorf("permanent error")
			},
		},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Failed != 1 {
		t.Errorf("Failed = %d, want 1", pr.Failed)
	}
	if pr.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", pr.Succeeded)
	}
	if pr.Results[0].Err == nil {
		t.Error("expected error in result")
	}
}

func TestParallelExecutor_MixedResults(t *testing.T) {
	pe := &ParallelExecutor{Concurrency: 3, RetryLimit: 2}

	jobs := []ParallelJob{
		{PerspectiveID: "ok1", Fn: func(ctx context.Context) (string, error) {
			return "/out/ok1", nil
		}},
		{PerspectiveID: "fail1", Fn: func(ctx context.Context) (string, error) {
			return "", fmt.Errorf("fail")
		}},
		{PerspectiveID: "ok2", Fn: func(ctx context.Context) (string, error) {
			return "/out/ok2", nil
		}},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", pr.Succeeded)
	}
	if pr.Failed != 1 {
		t.Errorf("Failed = %d, want 1", pr.Failed)
	}

	// Check ordering preserved
	if pr.Results[0].Err != nil {
		t.Error("Results[0] should succeed")
	}
	if pr.Results[1].Err == nil {
		t.Error("Results[1] should fail")
	}
	if pr.Results[2].Err != nil {
		t.Error("Results[2] should succeed")
	}
}

func TestParallelExecutor_EmptyJobs(t *testing.T) {
	pe := &ParallelExecutor{Concurrency: 4}
	pr := pe.Execute(context.Background(), nil)

	if pr.Succeeded != 0 || pr.Failed != 0 {
		t.Errorf("empty jobs should produce zero results, got succeeded=%d failed=%d", pr.Succeeded, pr.Failed)
	}
	if len(pr.Results) != 0 {
		t.Errorf("Results len = %d, want 0", len(pr.Results))
	}
}

func TestParallelExecutor_ContextCancellation(t *testing.T) {
	pe := &ParallelExecutor{Concurrency: 1}

	ctx, cancel := context.WithCancel(context.Background())

	var started int64
	jobs := []ParallelJob{
		{
			PerspectiveID: "blocker",
			Fn: func(ctx context.Context) (string, error) {
				atomic.AddInt64(&started, 1)
				// Block long enough for cancellation
				time.Sleep(200 * time.Millisecond)
				return "/out/blocker", nil
			},
		},
		{
			PerspectiveID: "waiter",
			Fn: func(ctx context.Context) (string, error) {
				atomic.AddInt64(&started, 1)
				return "/out/waiter", nil
			},
		},
	}

	// Cancel after blocker starts but before waiter can acquire semaphore
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	pr := pe.Execute(ctx, jobs)

	// At least the blocker ran; waiter may or may not depending on timing
	if len(pr.Results) != 2 {
		t.Fatalf("Results len = %d, want 2", len(pr.Results))
	}

	// We don't assert exact counts because timing-dependent,
	// but verify all results are populated
	if pr.Succeeded+pr.Failed != 2 {
		t.Errorf("total results = %d, want 2", pr.Succeeded+pr.Failed)
	}
}

func TestParallelExecutor_OnJobCompleteCallback(t *testing.T) {
	var mu sync.Mutex
	callbacks := make(map[string]bool)

	pe := &ParallelExecutor{
		Concurrency: 2,
		RetryLimit:  2,
		OnJobComplete: func(perspectiveID string, success bool, attempts int) {
			mu.Lock()
			callbacks[perspectiveID] = success
			mu.Unlock()
		},
	}

	jobs := []ParallelJob{
		{PerspectiveID: "ok", Fn: func(ctx context.Context) (string, error) {
			return "/out/ok", nil
		}},
		{PerspectiveID: "fail", Fn: func(ctx context.Context) (string, error) {
			return "", fmt.Errorf("fail")
		}},
	}

	pe.Execute(context.Background(), jobs)

	mu.Lock()
	defer mu.Unlock()

	if !callbacks["ok"] {
		t.Error("callback for 'ok' should report success=true")
	}
	if callbacks["fail"] {
		t.Error("callback for 'fail' should report success=false")
	}
}

func TestParallelExecutor_DefaultConcurrency(t *testing.T) {
	pe := &ParallelExecutor{} // zero value — should use default

	var active int64
	var maxActive int64

	jobs := make([]ParallelJob, 8)
	for i := range jobs {
		pid := fmt.Sprintf("p%d", i)
		jobs[i] = ParallelJob{
			PerspectiveID: pid,
			Fn: func(ctx context.Context) (string, error) {
				cur := atomic.AddInt64(&active, 1)
				for {
					old := atomic.LoadInt64(&maxActive)
					if cur <= old || atomic.CompareAndSwapInt64(&maxActive, old, cur) {
						break
					}
				}
				time.Sleep(30 * time.Millisecond)
				atomic.AddInt64(&active, -1)
				return "/out/" + pid, nil
			},
		}
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 8 {
		t.Errorf("Succeeded = %d, want 8", pr.Succeeded)
	}

	observed := atomic.LoadInt64(&maxActive)
	if observed > int64(DefaultConcurrencyLimit) {
		t.Errorf("max concurrent = %d, should be <= DefaultConcurrencyLimit (%d)", observed, DefaultConcurrencyLimit)
	}
}

func TestParallelExecutor_ConcurrencyCappedToJobCount(t *testing.T) {
	// If concurrency > job count, should cap to job count (no idle slots)
	pe := &ParallelExecutor{Concurrency: 100}

	jobs := []ParallelJob{
		{PerspectiveID: "p1", Fn: func(ctx context.Context) (string, error) {
			return "/out/p1", nil
		}},
		{PerspectiveID: "p2", Fn: func(ctx context.Context) (string, error) {
			return "/out/p2", nil
		}},
	}

	pr := pe.Execute(context.Background(), jobs)
	if pr.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", pr.Succeeded)
	}
}

func TestParallelExecutor_PerspectiveIDPreservedInResults(t *testing.T) {
	pe := &ParallelExecutor{Concurrency: 2, RetryLimit: 1}

	jobs := []ParallelJob{
		{PerspectiveID: "security", Fn: func(ctx context.Context) (string, error) {
			return "/findings/security", nil
		}},
		{PerspectiveID: "performance", Fn: func(ctx context.Context) (string, error) {
			return "", fmt.Errorf("permanent failure")
		}},
	}

	pr := pe.Execute(context.Background(), jobs)

	// PerspectiveID must be set on all results regardless of success/failure
	if pr.Results[0].PerspectiveID != "security" {
		t.Errorf("Results[0].PerspectiveID = %q, want 'security'", pr.Results[0].PerspectiveID)
	}
	if pr.Results[1].PerspectiveID != "performance" {
		t.Errorf("Results[1].PerspectiveID = %q, want 'performance'", pr.Results[1].PerspectiveID)
	}
}

func TestParallelExecutor_JobTimeoutEnforced(t *testing.T) {
	// Jobs that exceed JobTimeout should be cancelled via context
	pe := &ParallelExecutor{
		Concurrency: 2,
		RetryLimit:  1, // No retry — single attempt
		JobTimeout:  100 * time.Millisecond,
	}

	jobs := []ParallelJob{
		{
			PerspectiveID: "fast",
			Fn: func(ctx context.Context) (string, error) {
				return "/out/fast", nil
			},
		},
		{
			PerspectiveID: "slow",
			Fn: func(ctx context.Context) (string, error) {
				// Simulate a long-running subprocess that respects context
				select {
				case <-ctx.Done():
					return "", fmt.Errorf("timed out: %w", ctx.Err())
				case <-time.After(5 * time.Second):
					return "/out/slow", nil
				}
			},
		},
	}

	start := time.Now()
	pr := pe.Execute(context.Background(), jobs)
	elapsed := time.Since(start)

	// Fast job should succeed
	if pr.Results[0].Err != nil {
		t.Errorf("fast job should succeed, got: %v", pr.Results[0].Err)
	}

	// Slow job should fail due to timeout
	if pr.Results[1].Err == nil {
		t.Error("slow job should fail due to timeout")
	}

	// Total time should be near JobTimeout, not 5 seconds
	if elapsed > 2*time.Second {
		t.Errorf("execution took %v, expected near 100ms timeout — timeout not enforced", elapsed)
	}

	if pr.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", pr.Succeeded)
	}
	if pr.Failed != 1 {
		t.Errorf("Failed = %d, want 1", pr.Failed)
	}
}

func TestParallelExecutor_JobTimeoutPerAttempt(t *testing.T) {
	// Each retry attempt should get a fresh timeout, not share the first attempt's deadline.
	var attempts int64

	pe := &ParallelExecutor{
		Concurrency: 1,
		RetryLimit:  2,
		JobTimeout:  200 * time.Millisecond,
	}

	jobs := []ParallelJob{
		{
			PerspectiveID: "retry-timeout",
			Fn: func(ctx context.Context) (string, error) {
				a := atomic.AddInt64(&attempts, 1)
				if a == 1 {
					// First attempt: sleep just under timeout, then fail
					time.Sleep(50 * time.Millisecond)
					return "", fmt.Errorf("transient error")
				}
				// Second attempt: check that we have a fresh deadline
				deadline, ok := ctx.Deadline()
				if !ok {
					return "", fmt.Errorf("no deadline on retry context")
				}
				remaining := time.Until(deadline)
				// Should have close to full JobTimeout remaining (200ms), not just leftover from first attempt
				if remaining < 100*time.Millisecond {
					return "", fmt.Errorf("retry deadline too short: %v remaining", remaining)
				}
				return "/out/retry-timeout", nil
			},
		},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", pr.Succeeded)
	}
	if pr.Results[0].Err != nil {
		t.Errorf("job should succeed on retry, got: %v", pr.Results[0].Err)
	}
	if atomic.LoadInt64(&attempts) != 2 {
		t.Errorf("attempts = %d, want 2", atomic.LoadInt64(&attempts))
	}
}

func TestParallelExecutor_JobTimeoutDefaultApplied(t *testing.T) {
	// When JobTimeout is zero, DefaultJobTimeout should be applied.
	pe := &ParallelExecutor{
		Concurrency: 1,
		RetryLimit:  1,
		// JobTimeout: 0 — should use DefaultJobTimeout
	}

	jobs := []ParallelJob{
		{
			PerspectiveID: "check-default",
			Fn: func(ctx context.Context) (string, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					return "", fmt.Errorf("expected deadline from JobTimeout")
				}
				remaining := time.Until(deadline)
				// DefaultJobTimeout is 12 minutes, so remaining should be well above 10 minutes
				if remaining < 10*time.Minute {
					return "", fmt.Errorf("default timeout too short: %v", remaining)
				}
				return "/out/check-default", nil
			},
		},
	}

	pr := pe.Execute(context.Background(), jobs)
	if pr.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1; err: %v", pr.Succeeded, pr.Results[0].Err)
	}
}

func TestParallelExecutor_TimeoutWithConcurrencyLimit(t *testing.T) {
	// Verify that per-job timeout works correctly when jobs are queued
	// behind the semaphore. A job waiting for a semaphore slot should
	// still get the full timeout once it starts.
	pe := &ParallelExecutor{
		Concurrency: 1, // Only 1 at a time
		RetryLimit:  1,
		JobTimeout:  300 * time.Millisecond,
	}

	var startTimes sync.Map

	jobs := []ParallelJob{
		{
			PerspectiveID: "first",
			Fn: func(ctx context.Context) (string, error) {
				startTimes.Store("first", time.Now())
				time.Sleep(100 * time.Millisecond) // Takes 100ms
				return "/out/first", nil
			},
		},
		{
			PerspectiveID: "second",
			Fn: func(ctx context.Context) (string, error) {
				startTimes.Store("second", time.Now())
				// Check that we have close to a full timeout
				deadline, ok := ctx.Deadline()
				if !ok {
					return "", fmt.Errorf("no deadline")
				}
				remaining := time.Until(deadline)
				// Should have close to 300ms, not 200ms (300ms - first's 100ms)
				if remaining < 200*time.Millisecond {
					return "", fmt.Errorf("queued job got reduced timeout: %v", remaining)
				}
				return "/out/second", nil
			},
		},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Succeeded != 2 {
		for i, r := range pr.Results {
			if r.Err != nil {
				t.Errorf("job %d (%s) failed: %v", i, r.PerspectiveID, r.Err)
			}
		}
	}
}

func TestParallelExecutor_AllJobsTimeoutGracefully(t *testing.T) {
	// All jobs time out — should still return results for all, not hang
	pe := &ParallelExecutor{
		Concurrency: 4,
		RetryLimit:  1, // Don't retry timeouts
		JobTimeout:  50 * time.Millisecond,
	}

	jobs := make([]ParallelJob, 6)
	for i := range jobs {
		pid := fmt.Sprintf("timeout-%d", i)
		jobs[i] = ParallelJob{
			PerspectiveID: pid,
			Fn: func(ctx context.Context) (string, error) {
				select {
				case <-ctx.Done():
					return "", fmt.Errorf("timed out: %w", ctx.Err())
				case <-time.After(10 * time.Second):
					return "/should-not-reach", nil
				}
			},
		}
	}

	start := time.Now()
	pr := pe.Execute(context.Background(), jobs)
	elapsed := time.Since(start)

	if pr.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0 (all should timeout)", pr.Succeeded)
	}
	if pr.Failed != 6 {
		t.Errorf("Failed = %d, want 6", pr.Failed)
	}
	if len(pr.Results) != 6 {
		t.Fatalf("Results len = %d, want 6", len(pr.Results))
	}

	// Should complete quickly, not wait 10 seconds per job
	if elapsed > 3*time.Second {
		t.Errorf("execution took %v, expected quick completion — timeouts not working", elapsed)
	}
}
