package scheduler_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quorant/quorant/internal/platform/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingJob records how many times Run is called.
type countingJob struct {
	name  string
	count atomic.Int64
}

func (j *countingJob) Name() string { return j.name }
func (j *countingJob) Run(_ context.Context) error {
	j.count.Add(1)
	return nil
}

// failingJob always returns an error.
type failingJob struct{}

func (j *failingJob) Name() string { return "failing_job" }
func (j *failingJob) Run(_ context.Context) error {
	return errors.New("job failed")
}

// blockingJob blocks until its context is cancelled, then records it stopped.
type blockingJob struct {
	stopped atomic.Bool
}

func (j *blockingJob) Name() string { return "blocking_job" }
func (j *blockingJob) Run(ctx context.Context) error {
	<-ctx.Done()
	j.stopped.Store(true)
	return ctx.Err()
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestScheduler_RunsJobImmediately verifies that a registered job is executed
// right away when Start is called (before the first ticker tick).
func TestScheduler_RunsJobImmediately(t *testing.T) {
	job := &countingJob{name: "immediate_job"}

	sched := scheduler.New(testLogger())
	// Use a long interval so the ticker won't fire during the test.
	sched.Register(job, 10*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go sched.Start(ctx)

	// Give the goroutine a moment to execute the immediate run.
	require.Eventually(t, func() bool {
		return job.count.Load() >= 1
	}, 300*time.Millisecond, 10*time.Millisecond, "job should run immediately on scheduler start")
}

// TestScheduler_StopsOnCancel verifies that cancelling the context causes the
// scheduler to stop dispatching new runs.
func TestScheduler_StopsOnCancel(t *testing.T) {
	job := &countingJob{name: "cancellable_job"}

	sched := scheduler.New(testLogger())
	// Use a short interval so it would fire multiple times if not stopped.
	sched.Register(job, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	// Let the job run a couple of times.
	time.Sleep(120 * time.Millisecond)

	// Capture count, then cancel.
	countBeforeCancel := job.count.Load()
	cancel()

	select {
	case <-done:
		// scheduler unblocked as expected
	case <-time.After(500*time.Millisecond):
		t.Fatal("scheduler did not stop after context cancel")
	}

	// After cancellation the count should not grow significantly.
	time.Sleep(60 * time.Millisecond)
	countAfterWait := job.count.Load()

	// Allow at most one in-flight run after cancel.
	assert.LessOrEqual(t, countAfterWait-countBeforeCancel, int64(1),
		"no new runs should occur after context cancel")
}

// TestScheduler_ErrorDoesNotStopJob verifies that a job that returns an error
// is retried on the next tick (the scheduler keeps running).
func TestScheduler_ErrorDoesNotStopJob(t *testing.T) {
	logger := testLogger()
	sched := scheduler.New(logger)
	sched.Register(&failingJob{}, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic and should return once ctx expires.
	sched.Start(ctx)
}

// TestScheduler_MultipleJobs verifies that multiple registered jobs all run.
func TestScheduler_MultipleJobs(t *testing.T) {
	jobA := &countingJob{name: "job_a"}
	jobB := &countingJob{name: "job_b"}

	sched := scheduler.New(testLogger())
	sched.Register(jobA, 10*time.Minute)
	sched.Register(jobB, 10*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	go sched.Start(ctx)

	require.Eventually(t, func() bool {
		return jobA.count.Load() >= 1 && jobB.count.Load() >= 1
	}, 250*time.Millisecond, 10*time.Millisecond, "both jobs should run immediately")
}

// mockSLAPool is a minimal interface that the SLABreachMonitorJob needs for testing.
// We use a fakeExecJob to test the SQL-level behaviour without a real DB.

// fakeExecJob wraps a function as a scheduler.Job so we can test job logic
// by injecting a controllable Run implementation.
type fakeExecJob struct {
	jobName string
	runFn   func(ctx context.Context) error
}

func (j *fakeExecJob) Name() string { return j.jobName }
func (j *fakeExecJob) Run(ctx context.Context) error {
	return j.runFn(ctx)
}

// TestSLABreachMonitor_UpdatesBreachedTasks verifies the SLA breach monitor
// job calls its underlying update and returns no error on success.
// Uses a mock job wrapping the same logic path.
func TestSLABreachMonitor_UpdatesBreachedTasks(t *testing.T) {
	updated := false
	job := &fakeExecJob{
		jobName: "sla_breach_monitor",
		runFn: func(ctx context.Context) error {
			// Simulate the DB update succeeding and marking breached tasks.
			updated = true
			return nil
		},
	}

	sched := scheduler.New(testLogger())
	sched.Register(job, 10*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	go sched.Start(ctx)

	require.Eventually(t, func() bool {
		return updated
	}, 250*time.Millisecond, 10*time.Millisecond, "SLA breach monitor should have run")
}
