package models

import (
	"context"
	"log"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

type WorkoutSegment = db.WorkoutSegment

// SegmentEffort is a segment plus when its workout started, for leaderboards.
type SegmentEffort = db.ListBestSegmentsByNameRow

// WorkoutComparison is a workout with its segments keyed by distance name, so efforts line up across workouts.
type WorkoutComparison struct {
	Workout  *Workout                   `json:"workout"`
	Segments map[string]*WorkoutSegment `json:"segments"`
}

// WorkoutSegments errors if the workout doesn't belong to the user, so callers can tell "no such workout" from "no segments".
func (m *storeModel) WorkoutSegments(ctx context.Context, workoutID int32, userToken *tokens.UserToken) ([]*WorkoutSegment, error) {
	if _, err := m.Workout(ctx, workoutID, userToken); err != nil {
		return nil, err
	}
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	segments, err := querier.ListSegmentsByWorkout(ctx, &db.ListSegmentsByWorkoutParams{
		WorkoutID: workoutID,
		UserID:    userToken.ID,
	})
	if err != nil {
		log.Printf("Could not list segments for workout %d: %v", workoutID, err)
		return nil, err
	}
	return segments, nil
}

func (m *storeModel) BestSegments(ctx context.Context, name string, limit int32, userToken *tokens.UserToken) ([]*SegmentEffort, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	efforts, err := querier.ListBestSegmentsByName(ctx, &db.ListBestSegmentsByNameParams{
		UserID: userToken.ID,
		Name:   name,
		Limit:  limit,
	})
	if err != nil {
		log.Printf("Could not list best %s segments: %v", name, err)
		return nil, err
	}
	return efforts, nil
}

func (m *storeModel) CompareWorkouts(ctx context.Context, workoutIDs []int32, userToken *tokens.UserToken) ([]*WorkoutComparison, error) {
	comparisons := make([]*WorkoutComparison, 0, len(workoutIDs))
	for _, id := range workoutIDs {
		workout, err := m.Workout(ctx, id, userToken)
		if err != nil {
			return nil, err
		}
		segments, err := m.WorkoutSegments(ctx, id, userToken)
		if err != nil {
			return nil, err
		}
		byName := make(map[string]*WorkoutSegment, len(segments))
		for _, s := range segments {
			byName[s.Name] = s
		}
		comparisons = append(comparisons, &WorkoutComparison{Workout: workout, Segments: byName})
	}
	return comparisons, nil
}

// RecomputeSummary reports a bulk recompute; one bad workout doesn't stop the rest.
type RecomputeSummary struct {
	Recomputed int               `json:"recomputed"`
	Skipped    int               `json:"skipped"`
	Failed     []*RecomputeError `json:"failed"`
}

type RecomputeError struct {
	WorkoutID int32  `json:"workout_id"`
	Error     string `json:"error"`
}

// RecomputeSegments re-reads the workout's activity file and replaces its segments, for workouts from before segments existed or after the algorithm changes.
func (m *storeModel) RecomputeSegments(ctx context.Context, workoutID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) ([]*WorkoutSegment, error) {
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

	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	// Without transactions a failure below leaves the workout with no segments, which a retry fixes.
	if err := querier.DeleteSegmentsByWorkout(ctx, &db.DeleteSegmentsByWorkoutParams{WorkoutID: workoutID, UserID: userToken.ID}); err != nil {
		log.Printf("Could not delete segments for workout %d: %v", workoutID, err)
		return nil, err
	}
	if err := m.addSegments(ctx, workoutID, workouts.FastestSegments(run, workouts.StandardDistances), userToken); err != nil {
		return nil, err
	}
	return m.WorkoutSegments(ctx, workoutID, userToken)
}

func (m *storeModel) RecomputeAllSegments(ctx context.Context, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*RecomputeSummary, error) {
	all, err := m.Workouts(ctx, userToken)
	if err != nil {
		return nil, err
	}
	summary := &RecomputeSummary{Failed: []*RecomputeError{}}
	for _, w := range all {
		if w.ActivityFileID == nil {
			summary.Skipped++
			continue
		}
		if _, err := m.RecomputeSegments(ctx, w.ID, userToken, fileRepo); err != nil {
			summary.Failed = append(summary.Failed, &RecomputeError{WorkoutID: w.ID, Error: err.Error()})
			continue
		}
		summary.Recomputed++
	}
	return summary, nil
}
