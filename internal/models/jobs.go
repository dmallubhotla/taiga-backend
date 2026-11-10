package models

import (
	"context"
	"log"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/jackc/pgx/v5/pgtype"
)

type Job = db.Job

const (
	JobProcessWorkout = "process_workout"

	JobQueued  = "queued"
	JobRunning = "running"
	JobDone    = "done"
	JobDead    = "dead"
)

func timestamptz(t time.Time) pgtype.Timestamptz {
	// UTC throughout, so sqlite's text timestamps compare in time order.
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// EnqueueStaleWorkouts queues reprocessing for every user's file-backed workouts not built by the current workouts.ProcessorVersion.
func (m *storeModel) EnqueueStaleWorkouts(ctx context.Context, now time.Time) (int64, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return 0, err
	}
	return querier.EnqueueStaleWorkouts(ctx, &db.EnqueueStaleWorkoutsParams{
		ProcessorVersion: workouts.ProcessorVersion,
		RunAt:            timestamptz(now),
	})
}

// EnqueueReprocessAll queues reprocessing for all the user's file-backed workouts, current or not; ones already queued aren't counted.
func (m *storeModel) EnqueueReprocessAll(ctx context.Context, now time.Time, userToken *tokens.UserToken) (int64, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return 0, err
	}
	return querier.EnqueueUserWorkouts(ctx, &db.EnqueueUserWorkoutsParams{
		UserID: userToken.ID,
		RunAt:  timestamptz(now),
	})
}

// ClaimJob marks the next due job running until leaseUntil; it returns nil with no error when nothing is due.
func (m *storeModel) ClaimJob(ctx context.Context, now, leaseUntil time.Time) (*Job, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	job, err := querier.ClaimJob(ctx, &db.ClaimJobParams{Now: timestamptz(now), LeaseUntil: timestamptz(leaseUntil)})
	if IsNotFound(err) {
		return nil, nil
	}
	return job, err
}

func (m *storeModel) FinishJob(ctx context.Context, id int32, now time.Time) error {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return err
	}
	return querier.FinishJob(ctx, &db.FinishJobParams{ID: id, UpdatedAt: timestamptz(now)})
}

func (m *storeModel) RetryJobLater(ctx context.Context, id int32, runAt time.Time, jobErr error, now time.Time) error {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return err
	}
	msg := jobErr.Error()
	return querier.RetryJobLater(ctx, &db.RetryJobLaterParams{ID: id, RunAt: timestamptz(runAt), LastError: &msg, UpdatedAt: timestamptz(now)})
}

func (m *storeModel) KillJob(ctx context.Context, id int32, jobErr error, now time.Time) error {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return err
	}
	msg := jobErr.Error()
	return querier.KillJob(ctx, &db.KillJobParams{ID: id, LastError: &msg, UpdatedAt: timestamptz(now)})
}

// PruneJobs deletes jobs that finished before cutoff; dead jobs stay until retried.
func (m *storeModel) PruneJobs(ctx context.Context, cutoff time.Time) (int64, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return 0, err
	}
	return querier.DeleteDoneJobsBefore(ctx, timestamptz(cutoff))
}

// Jobs lists the user's newest jobs, optionally only those in state.
func (m *storeModel) Jobs(ctx context.Context, state string, limit int32, userToken *tokens.UserToken) ([]*Job, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	if state == "" {
		return querier.ListJobsByUser(ctx, &db.ListJobsByUserParams{UserID: userToken.ID, Limit: limit})
	}
	return querier.ListJobsByUserAndState(ctx, &db.ListJobsByUserAndStateParams{UserID: userToken.ID, State: state, Limit: limit})
}

func (m *storeModel) Job(ctx context.Context, id int32, userToken *tokens.UserToken) (*Job, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	return querier.GetJob(ctx, &db.GetJobParams{ID: id, UserID: userToken.ID})
}

// RetryDeadJob requeues a dead job with fresh attempts; ErrJobNotDead if it exists but isn't dead.
func (m *storeModel) RetryDeadJob(ctx context.Context, id int32, now time.Time, userToken *tokens.UserToken) (*Job, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	job, err := querier.RequeueDeadJob(ctx, &db.RequeueDeadJobParams{ID: id, UserID: userToken.ID, RunAt: timestamptz(now)})
	if IsNotFound(err) {
		if _, getErr := m.Job(ctx, id, userToken); getErr == nil {
			return nil, ErrJobNotDead
		}
	}
	if err != nil {
		log.Printf("Could not retry job %d: %v", id, err)
	}
	return job, err
}
