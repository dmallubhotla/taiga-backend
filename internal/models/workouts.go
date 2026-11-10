package models

import (
	"context"
	"log"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
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
	log.Printf("Deleting workout %d", id)
	err := m.store.InTx(ctx, func(q db.Querier) error {
		// SQLite doesn't enforce ON DELETE CASCADE without a per-connection pragma, so clear dependents explicitly.
		if err := q.DeleteSegmentsByWorkout(ctx, &db.DeleteSegmentsByWorkoutParams{WorkoutID: id, UserID: userToken.ID}); err != nil {
			return err
		}
		if err := q.DeleteJobsByWorkout(ctx, &db.DeleteJobsByWorkoutParams{WorkoutID: &id, UserID: userToken.ID}); err != nil {
			return err
		}
		return q.DeleteWorkout(ctx, &db.DeleteWorkoutParams{ID: id, UserID: userToken.ID})
	})
	if err != nil {
		log.Printf("Could not delete workout %d: %v", id, err)
	}
	return err
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
