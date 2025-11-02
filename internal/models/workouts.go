package models

import (
	"context"
	"log"

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

func (m *storeModel) CreateWorkoutFromActivityFile(ctx context.Context, activityFileID int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*Workout, error) {
	// Get the activity file content
	content, err := m.GetActivityFileContent(ctx, activityFileID, userToken, fileRepo)
	if err != nil {
		log.Printf("Could not get activity file content: %v", err)
		return nil, err
	}
	defer content.Close()

	// Parse the FIT file
	run, err := workouts.ReadFitFile(content)
	if err != nil {
		log.Printf("Could not parse FIT file: %v", err)
		return nil, err
	}

	// Convert RunSummary to Workout
	workout := &Workout{
		DistanceMiles:  &run.Summary.DistanceMiles,
		TimeSeconds:    &run.Summary.TimeSeconds,
		SpeedMph:       &run.Summary.SpeedMph,
		PaceMinPerMile: &run.Summary.PaceMinPerMile,
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

	// Save the workout
	return m.AddWorkout(ctx, workout, userToken)
}
