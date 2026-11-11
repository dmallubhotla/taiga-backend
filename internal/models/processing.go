package models

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/jackc/pgx/v5/pgtype"
)

// readRun parses an activity file's stored content.
func (m *storeModel) readRun(ctx context.Context, activityFileID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*workouts.Run, error) {
	content, err := m.GetActivityFileContent(ctx, activityFileID, userToken, fileRepo)
	if err != nil {
		log.Printf("Could not get activity file content: %v", err)
		return nil, err
	}
	defer func() { _ = content.Close() }()

	run, err := workouts.ReadFitFile(content)
	if err != nil {
		log.Printf("Could not parse FIT file: %v", err)
		return nil, err
	}
	return run, nil
}

func (m *storeModel) CreateWorkoutFromActivityFile(ctx context.Context, activityFileID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*Workout, bool, error) {
	existing, err := m.GetWorkoutsByActivityFile(ctx, activityFileID, userToken)
	if err != nil {
		return nil, false, err
	}
	if len(existing) > 0 {
		log.Printf("Activity file %d already has workout %d", activityFileID, existing[0].ID)
		return existing[0], false, nil
	}

	run, err := m.readRun(ctx, activityFileID, userToken, fileRepo)
	if err != nil {
		return nil, false, err
	}

	var saved *Workout
	err = m.store.InTx(ctx, func(q db.Querier) error {
		s := run.Summary
		saved, err = q.CreateWorkout(ctx, &db.CreateWorkoutParams{
			DistanceMiles:    finiteOrNil(s.DistanceMiles),
			TimeSeconds:      finiteOrNil(s.TimeSeconds),
			SpeedMph:         finiteOrNil(s.SpeedMph),
			PaceMinPerMile:   finiteOrNil(s.PaceMinPerMile),
			StartTime:        pgtype.Timestamptz{Time: s.StartTime, Valid: true},
			EndTime:          pgtype.Timestamptz{Time: s.EndTime, Valid: true},
			ActivityFileID:   &activityFileID,
			UserID:           userToken.ID,
			ProcessorVersion: workouts.ProcessorVersion,
		})
		if err != nil {
			return err
		}
		return storeDerived(ctx, q, saved.ID, run, userToken)
	})
	if err != nil {
		log.Printf("Could not create workout from activity file %d: %v", activityFileID, err)
		return nil, false, err
	}
	return saved, true, nil
}

// ReprocessWorkout rebuilds everything derived from the workout's activity file (summary, segments) and stamps the current ProcessorVersion.
func (m *storeModel) ReprocessWorkout(ctx context.Context, workoutID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*Workout, error) {
	workout, err := m.Workout(ctx, workoutID, userToken)
	if err != nil {
		return nil, err
	}
	if workout.ActivityFileID == nil {
		return nil, ErrNoActivityFile
	}
	run, err := m.readRun(ctx, *workout.ActivityFileID, userToken, fileRepo)
	if err != nil {
		return nil, err
	}

	var updated *Workout
	err = m.store.InTx(ctx, func(q db.Querier) error {
		s := run.Summary
		updated, err = q.UpdateWorkoutDerived(ctx, &db.UpdateWorkoutDerivedParams{
			ID:               workoutID,
			UserID:           userToken.ID,
			DistanceMiles:    finiteOrNil(s.DistanceMiles),
			TimeSeconds:      finiteOrNil(s.TimeSeconds),
			SpeedMph:         finiteOrNil(s.SpeedMph),
			PaceMinPerMile:   finiteOrNil(s.PaceMinPerMile),
			StartTime:        pgtype.Timestamptz{Time: s.StartTime, Valid: true},
			EndTime:          pgtype.Timestamptz{Time: s.EndTime, Valid: true},
			ProcessorVersion: workouts.ProcessorVersion,
		})
		if err != nil {
			return err
		}
		return storeDerived(ctx, q, workoutID, run, userToken)
	})
	if err != nil {
		log.Printf("Could not reprocess workout %d: %v", workoutID, err)
		return nil, err
	}
	return updated, nil
}

// storeDerived replaces everything computed from the run's records; replacing the whole segment set drops distances no longer in StandardDistances.
func storeDerived(ctx context.Context, q db.Querier, workoutID int32, run *workouts.Run, userToken *tokens.UserToken) error {
	if err := q.DeleteSegmentsByWorkout(ctx, &db.DeleteSegmentsByWorkoutParams{WorkoutID: workoutID, UserID: userToken.ID}); err != nil {
		return err
	}
	if err := addSegments(ctx, q, workoutID, run, userToken); err != nil {
		return err
	}
	series, err := json.Marshal(workouts.BuildSeries(run, workouts.MaxSeriesPoints))
	if err != nil {
		return fmt.Errorf("encoding series for workout %d: %w", workoutID, err)
	}
	return q.UpsertWorkoutSeries(ctx, &db.UpsertWorkoutSeriesParams{WorkoutID: workoutID, UserID: userToken.ID, Series: series})
}

// WorkoutSeries returns the stored series JSON; ErrNoSeries for a workout with nothing recorded (hand-entered, or not yet reprocessed).
func (m *storeModel) WorkoutSeries(ctx context.Context, workoutID int32, userToken *tokens.UserToken) ([]byte, error) {
	if _, err := m.Workout(ctx, workoutID, userToken); err != nil {
		return nil, err
	}
	querier, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	series, err := querier.GetWorkoutSeries(ctx, &db.GetWorkoutSeriesParams{WorkoutID: workoutID, UserID: userToken.ID})
	if IsNotFound(err) {
		return nil, ErrNoSeries
	}
	return series, err
}

func addSegments(ctx context.Context, q db.Querier, workoutID int32, run *workouts.Run, userToken *tokens.UserToken) error {
	for _, seg := range workouts.FastestSegments(run, workouts.StandardDistances) {
		_, err := q.CreateWorkoutSegment(ctx, &db.CreateWorkoutSegmentParams{
			WorkoutID:          workoutID,
			UserID:             userToken.ID,
			Name:               seg.Name,
			DistanceMeters:     seg.Distance.Meters,
			DurationSeconds:    seg.Duration.Seconds(),
			StartOffsetSeconds: seg.StartOffset().Seconds(),
			EndOffsetSeconds:   seg.EndOffset().Seconds(),
		})
		if err != nil {
			log.Printf("Could not create %s segment for workout %d: %v", seg.Name, workoutID, err)
			return err
		}
	}
	return nil
}

// Missing FIT fields decode as NaN and zero distance yields an infinite pace; neither survives JSON encoding, so store them as NULL.
func finiteOrNil(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
