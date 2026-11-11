package models

import (
	"context"
	"log"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
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

func (m *storeModel) BestSegments(ctx context.Context, name, sport string, limit int32, window Window, userToken *tokens.UserToken) ([]*SegmentEffort, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	from, to := window.bounds()
	efforts, err := querier.ListBestSegmentsByName(ctx, &db.ListBestSegmentsByNameParams{
		WindowFrom: from,
		WindowTo:   to,
		UserID:     userToken.ID,
		Name:       name,
		PageLimit:  limit,
		Sport:      sport,
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
