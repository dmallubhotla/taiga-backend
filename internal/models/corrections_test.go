package models_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

// A walk at 5:00/km (3.3 m/s) is plausible for a walker; uploaded as running it tops nothing walking until corrected.
var briskWalk = []testsupport.Split{{Meters: 3000, Seconds: 900}}

func TestCorrectedSportMovesBestEfforts(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	w := uploadWorkout(t, m, repo, alice, day1, briskWalk)
	require.Equal(t, workouts.SportRunning, w.Sport)

	corrected, err := m.CorrectWorkout(ctx, w.ID, models.Corrections{Sport: workouts.SportWalking}, alice, repo)
	require.NoError(t, err)
	assert.Equal(t, workouts.SportWalking, corrected.Sport)
	assert.Equal(t, workouts.SportWalking, corrected.SportOverride)

	runs, err := m.BestSegments(ctx, "1k", workouts.SportRunning, 10, alice)
	require.NoError(t, err)
	assert.Empty(t, runs)
	walks, err := m.BestSegments(ctx, "1k", workouts.SportWalking, 10, alice)
	require.NoError(t, err)
	require.Len(t, walks, 1)
	assert.Equal(t, w.ID, walks[0].WorkoutID)

	reprocessed, err := m.ReprocessWorkout(ctx, w.ID, alice, repo)
	require.NoError(t, err)
	assert.Equal(t, workouts.SportWalking, reprocessed.Sport, "the correction survives reprocessing")

	cleared, err := m.CorrectWorkout(ctx, w.ID, models.Corrections{}, alice, repo)
	require.NoError(t, err)
	assert.Equal(t, workouts.SportRunning, cleared.Sport, "clearing it goes back to the file's sport")
	assert.Empty(t, cleared.SportOverride)
}

func TestCorrectedToARideIsNotScored(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	w := uploadWorkout(t, m, repo, alice, day1, fourK)

	_, err := m.CorrectWorkout(ctx, w.ID, models.Corrections{Sport: "cycling"}, alice, repo)
	require.NoError(t, err)
	segments, err := m.WorkoutSegments(ctx, w.ID, alice)
	require.NoError(t, err)
	assert.Empty(t, segments)
}

func TestExcludedWorkoutLeavesBestEffortsAndRoutes(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	first := uploadWorkout(t, m, repo, alice, day1, fourK)
	uploadWorkout(t, m, repo, alice, day2, fourK)
	require.NoError(t, m.DetectRoutes(ctx, alice))
	routes, err := m.Routes(ctx, alice)
	require.NoError(t, err)
	require.Len(t, routes, 1)

	excluded, err := m.CorrectWorkout(ctx, first.ID, models.Corrections{ExcludedReason: "ran with a friend"}, alice, repo)
	require.NoError(t, err)
	assert.Equal(t, "ran with a friend", excluded.ExcludedReason)
	assert.Equal(t, "ran with a friend", excluded.UserExcludedReason)

	efforts, err := m.BestSegments(ctx, "1k", workouts.SportRunning, 10, alice)
	require.NoError(t, err)
	require.Len(t, efforts, 1)
	assert.NotEqual(t, first.ID, efforts[0].WorkoutID)

	require.NoError(t, m.DetectRoutes(ctx, alice))
	routes, err = m.Routes(ctx, alice)
	require.NoError(t, err)
	assert.Empty(t, routes, "one run left is no route")

	listed, err := m.Workouts(ctx, alice)
	require.NoError(t, err)
	assert.Len(t, listed, 2, "an excluded workout is still listed")
}

func TestHandEnteredWorkoutsHaveNothingToCorrect(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	w, err := m.AddWorkout(ctx, &models.Workout{}, alice)
	require.NoError(t, err)

	_, err = m.CorrectWorkout(ctx, w.ID, models.Corrections{Sport: workouts.SportWalking}, alice, repo)
	assert.ErrorIs(t, err, models.ErrNoActivityFile)
}

func TestImplausibleEffortsAreNotStored(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	// A 1:40 kilometre (10 m/s) inside an otherwise ordinary run: a GPS jump, not an effort.
	w := uploadWorkout(t, m, repo, alice, day1, []testsupport.Split{{Meters: 1000, Seconds: 100}, {Meters: 4000, Seconds: 1200}})

	segments, err := m.WorkoutSegments(ctx, w.ID, alice)
	require.NoError(t, err)
	byName := segmentsByName(segments)
	assert.Nil(t, byName["1k"], "faster than the world record")
	require.NotNil(t, byName["5k"], "the whole run averages a plausible 4:20/km")
}
