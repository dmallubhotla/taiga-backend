package models

import (
	"context"
	"log"
	"math"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/jackc/pgx/v5/pgtype"
)

type Workout = db.Workout

func (m *storeModel) Workout(ctx context.Context, id int32, userToken *tokens.UserToken) (*Workout, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	getWorkoutParams := &db.GetWorkoutParams{
		ID:     id,
		UserID: userToken.ID,
	}
	workout, err := querier.GetWorkout(ctx, getWorkoutParams)
	if err != nil {
		log.Printf("Could not get workout: %v", err)
		return nil, err
	}

	return (*Workout)(workout), nil
}

func (m *storeModel) Workouts(ctx context.Context, userToken *tokens.UserToken) ([]*Workout, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	workouts, err := querier.ListWorkoutsByUser(ctx, userToken.ID)
	if err != nil {
		log.Printf("Could not get workouts: %v", err)
		return nil, err
	}
	log.Printf("Got %d workouts", len(workouts))
	var retWorkouts []*Workout
	for _, w := range workouts {
		retWorkouts = append(retWorkouts, (*Workout)(w))
	}

	return retWorkouts, nil
}

func (m *storeModel) AddWorkout(ctx context.Context, workout *Workout, userToken *tokens.UserToken) (*Workout, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}

	createWorkoutParams := &db.CreateWorkoutParams{
		DistanceMiles:  workout.DistanceMiles,
		TimeSeconds:    workout.TimeSeconds,
		SpeedMph:       workout.SpeedMph,
		PaceMinPerMile: workout.PaceMinPerMile,
		StartTime:      workout.StartTime,
		EndTime:        workout.EndTime,
		ActivityFileID: workout.ActivityFileID,
		UserID:         userToken.ID,
	}
	log.Printf("Adding workout %v", createWorkoutParams)
	returnWorkout, err := querier.CreateWorkout(ctx, createWorkoutParams)
	if err != nil {
		log.Printf("Could not create workout: %v", err)
		return nil, err
	}
	return returnWorkout, nil
}

func (m *storeModel) UpdateWorkout(ctx context.Context, id int32, workout *Workout, userToken *tokens.UserToken) (*Workout, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}

	updateWorkoutParams := &db.UpdateWorkoutParams{
		ID:             id,
		UserID:         userToken.ID,
		DistanceMiles:  workout.DistanceMiles,
		TimeSeconds:    workout.TimeSeconds,
		SpeedMph:       workout.SpeedMph,
		PaceMinPerMile: workout.PaceMinPerMile,
		StartTime:      workout.StartTime,
		EndTime:        workout.EndTime,
		ActivityFileID: workout.ActivityFileID,
	}
	log.Printf("Updating workout %v", updateWorkoutParams)
	returnWorkout, err := querier.UpdateWorkout(ctx, updateWorkoutParams)
	if err != nil {
		log.Printf("Could not update workout: %v", err)
		return nil, err
	}
	return returnWorkout, nil
}

func (m *storeModel) DeleteWorkout(ctx context.Context, id int32, userToken *tokens.UserToken) error {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return err
	}

	// SQLite doesn't enforce ON DELETE CASCADE without a per-connection pragma, so clear segments explicitly.
	err = querier.DeleteSegmentsByWorkout(ctx, &db.DeleteSegmentsByWorkoutParams{
		WorkoutID: id,
		UserID:    userToken.ID,
	})
	if err != nil {
		log.Printf("Could not delete segments for workout %d: %v", id, err)
		return err
	}

	deleteWorkoutParams := &db.DeleteWorkoutParams{
		ID:     id,
		UserID: userToken.ID,
	}
	log.Printf("Deleting workout %v", deleteWorkoutParams)
	err = querier.DeleteWorkout(ctx, deleteWorkoutParams)
	if err != nil {
		log.Printf("Could not delete workout: %v", err)
		return err
	}
	return nil
}

func (m *storeModel) GetWorkoutsByActivityFile(ctx context.Context, activityFileID int32, userToken *tokens.UserToken) ([]*Workout, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}

	getWorkoutsByActivityFileParams := &db.GetWorkoutsByActivityFileParams{
		ActivityFileID: &activityFileID,
		UserID:         userToken.ID,
	}
	workouts, err := querier.GetWorkoutsByActivityFile(ctx, getWorkoutsByActivityFileParams)
	if err != nil {
		log.Printf("Could not get workouts by activity file: %v", err)
		return nil, err
	}

	var retWorkouts []*Workout
	for _, w := range workouts {
		retWorkouts = append(retWorkouts, (*Workout)(w))
	}

	return retWorkouts, nil
}

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

	// Convert RunSummary to Workout
	workout := &Workout{
		DistanceMiles:  finiteOrNil(run.Summary.DistanceMiles),
		TimeSeconds:    finiteOrNil(run.Summary.TimeSeconds),
		SpeedMph:       finiteOrNil(run.Summary.SpeedMph),
		PaceMinPerMile: finiteOrNil(run.Summary.PaceMinPerMile),
		StartTime: pgtype.Timestamptz{
			Time:  run.Summary.StartTime,
			Valid: true,
		},
		EndTime: pgtype.Timestamptz{
			Time:  run.Summary.EndTime,
			Valid: true,
		},
		ActivityFileID: &activityFileID,
	}

	saved, err := m.AddWorkout(ctx, workout, userToken)
	if err != nil {
		return nil, false, err
	}

	if err := m.addSegments(ctx, saved.ID, workouts.FastestSegments(run, workouts.StandardDistances), userToken); err != nil {
		// No transactions through the store yet, so undo by hand rather than leave a workout missing segments.
		if delErr := m.DeleteWorkout(ctx, saved.ID, userToken); delErr != nil {
			log.Printf("Could not roll back workout %d after segment failure: %v", saved.ID, delErr)
		}
		return nil, false, err
	}
	return saved, true, nil
}

func (m *storeModel) addSegments(ctx context.Context, workoutID int32, segments []*workouts.BestSegment, userToken *tokens.UserToken) error {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return err
	}
	for _, seg := range segments {
		_, err := querier.CreateWorkoutSegment(ctx, &db.CreateWorkoutSegmentParams{
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
