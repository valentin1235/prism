package parallel

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/heechul/prism-mcp/internal/engine"
)

// DefaultConcurrencyLimit is the default maximum number of concurrent
// claude CLI subprocesses for parallel stages (specialist/interview).
// This prevents overwhelming the system when many perspectives are active.
const DefaultConcurrencyLimit = 4

// ParallelJob represents a single unit of work for the parallel executor.
// Each job wraps a function that spawns a claude CLI subprocess and returns
// an output path and potential error. The PerspectiveID is used for logging
// and result correlation.
type ParallelJob struct {
	// PerspectiveID identifies the perspective this job belongs to.
	PerspectiveID string

	// Fn is the work function to execute. It receives a context for cancellation
	// and returns an output path and potential error.
	Fn func(ctx context.Context) (outputPath string, err error)
}

// DefaultJobTimeout is the default per-process timeout for claude CLI subprocesses.
// Each specialist/interview job gets this timeout for a single attempt.
// This acts as a safety net in addition to any timeout the job function itself may set.
const DefaultJobTimeout = 12 * time.Minute

// ParallelExecutor runs N jobs concurrently with a configurable concurrency
// limit using a semaphore pattern (buffered channel). Each job runs in its
// own goroutine, but at most `concurrency` goroutines are active at once.
//
// Features:
//   - Semaphore-based concurrency limiting via buffered channel
//   - Per-process timeout via context.WithTimeout for each job attempt
//   - Automatic retry (once) on failure per job
//   - Progress callback for real-time status updates
//   - Context propagation for cancellation
//   - stdout/stderr captured by underlying QueryLLM* functions
type ParallelExecutor struct {
	// Concurrency is the maximum number of concurrent jobs.
	// Defaults to DefaultConcurrencyLimit if <= 0.
	Concurrency int

	// OnJobComplete is called after each job finishes (success or final failure).
	// It receives the perspective ID, whether it succeeded, and the attempt count.
	// Called from the job's goroutine — must be thread-safe.
	OnJobComplete func(perspectiveID string, success bool, attempts int)

	// RetryLimit is the maximum number of attempts per job (including the first).
	// Defaults to 5 (mirrors Python _MAX_RETRIES = 5) if <= 0.
	RetryLimit int

	// InitialBackoff is the base delay before the first retry.
	// Subsequent retries use exponential backoff: InitialBackoff * 2^attempt.
	// Defaults to 2 seconds if <= 0 (mirrors Python _INITIAL_BACKOFF_SECONDS).
	InitialBackoff time.Duration

	// JobTimeout is the per-process timeout for each individual job attempt.
	// If a single attempt exceeds this duration, the job's context is cancelled.
	// Defaults to DefaultJobTimeout if <= 0.
	// The timeout applies independently to each retry attempt — a job that
	// times out on attempt 1 gets a fresh timeout for attempt 2.
	JobTimeout time.Duration
}

// JobResult holds the outcome of a single parallel job.
type JobResult struct {
	PerspectiveID string
	OutputPath    string
	Err           error
}

// ParallelResults holds the collected results from a parallel execution run.
type ParallelResults struct {
	Results   []JobResult
	Succeeded int
	Failed    int
}

// Execute runs all jobs with the configured concurrency limit and returns
// collected results. Results are indexed in the same order as the input jobs.
// Blocks until all jobs complete.
//
// The context is passed to each job function for cancellation support.
// If the context is cancelled, in-flight jobs will receive the cancellation
// but already-started jobs may still complete.
func (pe *ParallelExecutor) Execute(ctx context.Context, jobs []ParallelJob) ParallelResults {
	concurrency := pe.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrencyLimit
	}

	retryLimit := pe.RetryLimit
	if retryLimit <= 0 {
		retryLimit = 5 // mirrors Python _MAX_RETRIES = 5
	}

	initialBackoff := pe.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = 2 * time.Second // mirrors Python _INITIAL_BACKOFF_SECONDS
	}

	jobTimeout := pe.JobTimeout
	if jobTimeout <= 0 {
		jobTimeout = DefaultJobTimeout
	}

	n := len(jobs)
	if n == 0 {
		return ParallelResults{}
	}

	// Cap concurrency to the number of jobs — no point having idle slots
	if concurrency > n {
		concurrency = n
	}

	results := make([]JobResult, n)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j ParallelJob) {
			defer wg.Done()

			// Acquire semaphore slot
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[idx] = JobResult{
					PerspectiveID: j.PerspectiveID,
					Err:           fmt.Errorf("context cancelled before start: %w", ctx.Err()),
				}
				if pe.OnJobComplete != nil {
					pe.OnJobComplete(j.PerspectiveID, false, 0)
				}
				return
			}
			defer func() { <-sem }() // Release semaphore slot

			var result JobResult
			var attempts int

			for attempt := 1; attempt <= retryLimit; attempt++ {
				attempts = attempt

				// Check parent context before each attempt
				if ctx.Err() != nil {
					result = JobResult{
						PerspectiveID: j.PerspectiveID,
						Err:           fmt.Errorf("context cancelled: %w", ctx.Err()),
					}
					break
				}

				// Create per-attempt timeout context derived from the parent.
				// Each retry gets a fresh timeout — a timeout on attempt 1 doesn't
				// reduce the time available for attempt 2.
				attemptCtx, attemptCancel := context.WithTimeout(ctx, jobTimeout)
				outputPath, err := j.Fn(attemptCtx)
				attemptCancel() // release timer resources immediately
				result = JobResult{
					PerspectiveID: j.PerspectiveID,
					OutputPath:    outputPath,
					Err:           err,
				}

				if result.Err == nil {
					break // Success
				}

				// Annotate timeout errors for clearer diagnostics
				if attemptCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
					log.Printf("[parallel] Job %s timed out after %v (attempt %d/%d)",
						j.PerspectiveID, jobTimeout, attempt, retryLimit)
				}

				if attempt < retryLimit {
					// Apply exponential backoff for retryable/timeout errors,
					// immediate retry for others (mirrors Python pattern).
					if engine.IsRetryableError(result.Err) ||
						attemptCtx.Err() == context.DeadlineExceeded {
						backoff := time.Duration(float64(initialBackoff) * math.Pow(2, float64(attempt-1)))
						log.Printf("[parallel] Job %s failed (attempt %d/%d): %v — retrying in %v",
							j.PerspectiveID, attempt, retryLimit, result.Err, backoff)
						select {
						case <-time.After(backoff):
						case <-ctx.Done():
							result.Err = fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
							break
						}
					} else {
						log.Printf("[parallel] Job %s failed (attempt %d/%d): %v — retrying immediately",
							j.PerspectiveID, attempt, retryLimit, result.Err)
					}
				} else {
					// Wrap the final error to indicate retry exhaustion so downstream
					// classifiers (classifyError / classifyInterviewError) can detect it.
					result.Err = fmt.Errorf("all attempts failed for %s (tried %d times): %w",
						j.PerspectiveID, retryLimit, result.Err)
					log.Printf("[parallel] Job %s failed (attempt %d/%d): %v — no more retries",
						j.PerspectiveID, attempt, retryLimit, result.Err)
				}
			}

			results[idx] = result

			if pe.OnJobComplete != nil {
				pe.OnJobComplete(j.PerspectiveID, result.Err == nil, attempts)
			}
		}(i, job)
	}

	wg.Wait()

	// Tally results
	pr := ParallelResults{Results: results}
	for _, r := range results {
		if r.Err != nil {
			pr.Failed++
		} else {
			pr.Succeeded++
		}
	}

	return pr
}
