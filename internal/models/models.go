package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/store"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/jackc/pgx/v5"
)

var ErrNoFileContent = errors.New("activity file has no file content")

// ErrActivityFileInUse means workouts still reference the activity file.
var ErrActivityFileInUse = errors.New("activity file still has workouts; delete them first")

// ErrNoActivityFile means a workout was entered by hand, so there's nothing to reprocess it from.
var ErrNoActivityFile = errors.New("workout has no activity file")

// ErrDerivedWorkout means an edit would change fields that reprocessing owns.
var ErrDerivedWorkout = errors.New("workout comes from an activity file, so its summary is rebuilt from the file; reprocess it instead of editing")

// ErrJobNotDead means only a dead job can be retried.
var ErrJobNotDead = errors.New("only dead jobs can be retried")

// IsNotFound reports a missing row from either store: the sqlite wrapper surfaces database/sql's ErrNoRows, the postgres store pgx's.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

type Model interface {
	Healthy(ctx context.Context) error
	Close() error
	CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error)
	VerifyUserByEmailPassword(ctx context.Context, email string, password string) (*UserNoPassword, error)
	Hat(ctx context.Context, hatID int32, userToken *tokens.UserToken) (*Hat, error)
	Hats(ctx context.Context, userToken *tokens.UserToken) ([]*Hat, error)
	AddHat(ctx context.Context, hat *Hat, userToken *tokens.UserToken) (*Hat, error)
	ActivityFile(ctx context.Context, id int32, userToken *tokens.UserToken) (*ActivityFile, error)
	ActivityFiles(ctx context.Context, userToken *tokens.UserToken) ([]*ActivityFile, error)
	ActivityFilesWithoutWorkout(ctx context.Context, userToken *tokens.UserToken) ([]*ActivityFile, error)
	// AddActivityFile returns the user's existing file, with created false, when they've already uploaded the same content.
	AddActivityFile(ctx context.Context, fileReader io.Reader, timestamp time.Time, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (af *ActivityFile, created bool, err error)
	UpdateActivityFile(ctx context.Context, id int32, timestamp time.Time, fileRepoHash *string, userToken *tokens.UserToken) (*ActivityFile, error)
	DeleteActivityFile(ctx context.Context, id int32, userToken *tokens.UserToken) error
	GetActivityFileContent(ctx context.Context, id int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (io.ReadCloser, error)
	Workout(ctx context.Context, id int32, userToken *tokens.UserToken) (*Workout, error)
	Workouts(ctx context.Context, userToken *tokens.UserToken) ([]*Workout, error)
	AddWorkout(ctx context.Context, workout *Workout, userToken *tokens.UserToken) (*Workout, error)
	UpdateWorkout(ctx context.Context, id int32, workout *Workout, userToken *tokens.UserToken) (*Workout, error)
	DeleteWorkout(ctx context.Context, id int32, userToken *tokens.UserToken) error
	GetWorkoutsByActivityFile(ctx context.Context, activityFileID int32, userToken *tokens.UserToken) ([]*Workout, error)
	// CreateWorkoutFromActivityFile returns the file's existing workout, with created false, if it already has one.
	CreateWorkoutFromActivityFile(ctx context.Context, activityFileID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (w *Workout, created bool, err error)
	WorkoutSegments(ctx context.Context, workoutID int32, userToken *tokens.UserToken) ([]*WorkoutSegment, error)
	BestSegments(ctx context.Context, name string, limit int32, userToken *tokens.UserToken) ([]*SegmentEffort, error)
	CompareWorkouts(ctx context.Context, workoutIDs []int32, userToken *tokens.UserToken) ([]*WorkoutComparison, error)
	// ReprocessWorkout rebuilds the workout's summary and segments from its activity file.
	ReprocessWorkout(ctx context.Context, workoutID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*Workout, error)
	EnqueueReprocessAll(ctx context.Context, now time.Time, userToken *tokens.UserToken) (int64, error)
	Jobs(ctx context.Context, state string, limit int32, userToken *tokens.UserToken) ([]*Job, error)
	Job(ctx context.Context, id int32, userToken *tokens.UserToken) (*Job, error)
	RetryDeadJob(ctx context.Context, id int32, now time.Time, userToken *tokens.UserToken) (*Job, error)
	// The worker's side of the queue, across all users.
	EnqueueStaleWorkouts(ctx context.Context, now time.Time) (int64, error)
	ClaimJob(ctx context.Context, now, leaseUntil time.Time) (*Job, error)
	FinishJob(ctx context.Context, id int32, now time.Time) error
	RetryJobLater(ctx context.Context, id int32, runAt time.Time, jobErr error, now time.Time) error
	KillJob(ctx context.Context, id int32, jobErr error, now time.Time) error
	PruneJobs(ctx context.Context, cutoff time.Time) (int64, error)
}

type storeModel struct {
	store store.Store
}

func New(cfg *config.Config) (Model, error) {

	s, err := store.GetStore(cfg)
	if err != nil {
		if s != nil {
			if closeErr := s.Close(); closeErr != nil {
				log.Printf("error closing store after init failure: %v", closeErr)
			}
		}
		return nil, fmt.Errorf("failed to initialize database :%w", err)
	}

	return &storeModel{
		store: s,
	}, nil
}

// mostly for convenience for testing
func NewFromStore(s store.Store) Model {
	return &storeModel{
		store: s,
	}
}

func (m *storeModel) Close() error {
	return m.store.Close()
}

func (m *storeModel) Healthy(ctx context.Context) error {
	return m.store.Healthy(ctx)
}
