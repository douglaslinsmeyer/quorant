package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job is a periodic task that runs on a schedule.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// Scheduler runs jobs on fixed intervals.
type Scheduler struct {
	jobs   []entry
	logger *slog.Logger
}

type entry struct {
	job      Job
	interval time.Duration
}

// New creates a new Scheduler with the provided logger.
func New(logger *slog.Logger) *Scheduler {
	return &Scheduler{logger: logger}
}

// Register adds a job to run at the given interval.
func (s *Scheduler) Register(job Job, interval time.Duration) {
	s.jobs = append(s.jobs, entry{job: job, interval: interval})
}

// Start begins all registered jobs. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	for _, e := range s.jobs {
		e := e
		go func() {
			s.logger.Info("scheduler started", "job", e.job.Name(), "interval", e.interval)
			ticker := time.NewTicker(e.interval)
			defer ticker.Stop()

			// Run immediately on start
			if err := e.job.Run(ctx); err != nil {
				s.logger.Error("scheduler job failed", "job", e.job.Name(), "error", err)
			}

			for {
				select {
				case <-ctx.Done():
					s.logger.Info("scheduler stopped", "job", e.job.Name())
					return
				case <-ticker.C:
					if err := e.job.Run(ctx); err != nil {
						s.logger.Error("scheduler job failed", "job", e.job.Name(), "error", err)
					}
				}
			}
		}()
	}

	<-ctx.Done()
}
