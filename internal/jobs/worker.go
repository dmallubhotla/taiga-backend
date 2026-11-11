// Package jobs runs queued background work (the jobs table) inside the server process.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

const (
	// MaxAttempts before a job that keeps failing goes dead.
	MaxAttempts = 5
	// Lease is how long a claimed job may run before another worker may assume this one crashed and claim it again.
	Lease        = 5 * time.Minute
	baseBackoff  = 30 * time.Second
	pollInterval = 2 * time.Second
	keepDone     = 7 * 24 * time.Hour
	pruneEvery   = time.Hour
)

type Worker struct {
	m        models.Model
	fileRepo filerepo.FileRepo
	// Now is the clock, swappable in tests.
	Now func() time.Time
}

func NewWorker(m models.Model, fileRepo filerepo.FileRepo) *Worker {
	return &Worker{m: m, fileRepo: fileRepo, Now: time.Now}
}

// Run enqueues stale workouts, then works the queue until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	if n, err := w.m.EnqueueStaleWorkouts(ctx, w.Now()); err != nil {
		log.Printf("jobs: could not enqueue stale workouts: %v", err)
	} else if n > 0 {
		log.Printf("jobs: enqueued %d workouts for reprocessing (processor %s)", n, workouts.ProcessorVersion)
	}

	var lastPrune time.Time
	for {
		if w.Now().Sub(lastPrune) > pruneEvery {
			if _, err := w.m.PruneJobs(ctx, w.Now().Add(-keepDone)); err != nil {
				log.Printf("jobs: could not prune finished jobs: %v", err)
			}
			lastPrune = w.Now()
		}
		// Drain what's due before sleeping.
		for ctx.Err() == nil {
			found, err := w.RunOnce(ctx)
			if err != nil {
				log.Printf("jobs: %v", err)
				break
			}
			if !found {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// RunOnce claims and runs one due job, reporting whether there was one.
// The error is about the queue itself; a job's own failure is recorded on the job.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	// Once claimed, a job runs to completion even if ctx is cancelled for shutdown, rather than sitting "running" until its lease expires.
	ctx = context.WithoutCancel(ctx)
	now := w.Now()
	job, err := w.m.ClaimJob(ctx, now, now.Add(Lease))
	if err != nil {
		return false, fmt.Errorf("claiming a job: %w", err)
	}
	if job == nil {
		return false, nil
	}

	jobErr := w.run(ctx, job)
	now = w.Now()
	switch {
	case jobErr == nil:
		err = w.m.FinishJob(ctx, job.ID, now)
	case permanent(jobErr) || job.Attempts >= MaxAttempts:
		log.Printf("jobs: job %d (%s) is dead after %d attempts: %v", job.ID, job.Kind, job.Attempts, jobErr)
		err = w.m.KillJob(ctx, job.ID, jobErr, now)
	default:
		retryAt := now.Add(baseBackoff << (job.Attempts - 1))
		log.Printf("jobs: job %d (%s) failed, retrying at %v: %v", job.ID, job.Kind, retryAt, jobErr)
		err = w.m.RetryJobLater(ctx, job.ID, retryAt, jobErr, now)
	}
	if err != nil {
		return true, fmt.Errorf("recording the outcome of job %d: %w", job.ID, err)
	}
	return true, nil
}

var errUnknownKind = errors.New("unknown job kind")

func (w *Worker) run(ctx context.Context, job *models.Job) error {
	// Jobs act for their owner, so every query stays scoped to that user.
	user := &tokens.UserToken{ID: job.UserID}
	switch job.Kind {
	case models.JobProcessWorkout:
		if job.WorkoutID == nil {
			return fmt.Errorf("%w: %s job without a workout", errUnknownKind, job.Kind)
		}
		if _, err := w.m.ReprocessWorkout(ctx, *job.WorkoutID, user, w.fileRepo); err != nil {
			return err
		}
		// Queued behind any reprocessing still due, so a bulk reprocess regroups routes once at the end.
		return w.m.EnqueueDetectRoutes(ctx, w.Now(), user)
	case models.JobDetectRoutes:
		return w.m.DetectRoutes(ctx, user)
	default:
		return fmt.Errorf("%w: %q", errUnknownKind, job.Kind)
	}
}

// permanent errors would fail the same way on every retry.
func permanent(err error) bool {
	return models.IsNotFound(err) ||
		errors.Is(err, errUnknownKind) ||
		errors.Is(err, workouts.ErrUnreadableFile) ||
		errors.Is(err, workouts.ErrUnsupportedActivity) ||
		errors.Is(err, models.ErrNoFileContent) ||
		errors.Is(err, models.ErrNoActivityFile)
}
