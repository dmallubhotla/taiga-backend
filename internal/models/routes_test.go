package models_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/muktihari/fit/profile/typedef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

const (
	north = 0.0
	east  = 90.0
)

// uploadHeading uploads a 3km run heading the given bearing, taking secondsPerKm.
func uploadHeading(t *testing.T, m models.Model, repo filerepo.FileRepo, user *tokens.UserToken, day int, bearing float64, secondsPerKm float64) *models.Workout {
	t.Helper()
	ctx := context.Background()
	start := time.Date(2026, 11, day, 7, 0, 0, 0, time.UTC)
	splits := []testsupport.Split{{Meters: 3000, Seconds: 3 * secondsPerKm}}
	af, _, err := m.AddActivityFile(ctx, bytes.NewReader(testsupport.EncodeFITHeading(t, start, splits, bearing)), start, user, repo)
	require.NoError(t, err)
	w, _, err := m.CreateWorkoutFromActivityFile(ctx, af.ID, user, repo)
	require.NoError(t, err)
	return w
}

func TestDetectRoutes(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	bob := createUser(t, m, "bob@example.com")

	first := uploadHeading(t, m, repo, alice, 1, north, 300)
	lone := uploadHeading(t, m, repo, alice, 2, east, 300)
	fastest := uploadHeading(t, m, repo, alice, 3, north, 250)
	latest := uploadHeading(t, m, repo, alice, 4, north, 280)
	uploadHeading(t, m, repo, bob, 1, north, 300)
	uploadHeading(t, m, repo, bob, 2, north, 300)

	require.NoError(t, m.DetectRoutes(ctx, alice))
	routes, err := m.Routes(ctx, alice)
	require.NoError(t, err)
	require.Len(t, routes, 1, "the lone eastward run is not a route yet")
	r := routes[0]
	assert.Equal(t, 3, r.RunCount)
	assert.Equal(t, fastest.ID, *r.BestWorkoutID)
	assert.InDelta(t, 750, *r.BestTimeSeconds, 1e-6)
	assert.Equal(t, latest.StartTime.Time, r.LatestStartTime.Time)
	assert.Nil(t, r.Name)

	detail, err := m.Route(ctx, r.ID, alice)
	require.NoError(t, err)
	require.Len(t, detail.Workouts, 3)
	assert.Equal(t, latest.ID, detail.Workouts[0].ID, "newest first")
	assert.NotEmpty(t, detail.Track)

	onRoute, err := m.WorkoutRoute(ctx, first.ID, alice)
	require.NoError(t, err)
	assert.Equal(t, r.ID, onRoute.ID)
	_, err = m.WorkoutRoute(ctx, lone.ID, alice)
	assert.ErrorIs(t, err, models.ErrNoRoute)

	_, err = m.Route(ctx, r.ID, bob)
	assert.True(t, models.IsNotFound(err), "bob can't see alice's route: %v", err)
	require.NoError(t, m.DetectRoutes(ctx, bob))
	bobs, err := m.Routes(ctx, bob)
	require.NoError(t, err)
	require.Len(t, bobs, 1, "runs are never matched across users")
	assert.Equal(t, 2, bobs[0].RunCount)

	t.Run("a rebuild keeps the route's id and name", func(t *testing.T) {
		name := "River north"
		renamed, err := m.RenameRoute(ctx, r.ID, &name, alice)
		require.NoError(t, err)
		assert.Equal(t, name, *renamed.Name)

		uploadHeading(t, m, repo, alice, 5, east, 300)
		require.NoError(t, m.DetectRoutes(ctx, alice))
		routes, err := m.Routes(ctx, alice)
		require.NoError(t, err)
		require.Len(t, routes, 2)
		assert.Equal(t, 2, routes[0].RunCount, "the eastward route, run most recently")
		assert.Nil(t, routes[0].Name)
		assert.Equal(t, r.ID, routes[1].ID)
		assert.Equal(t, name, *routes[1].Name)
	})

	t.Run("deleting runs shrinks and then dissolves a route", func(t *testing.T) {
		require.NoError(t, m.DeleteWorkout(ctx, fastest.ID, alice))
		require.NoError(t, m.DetectRoutes(ctx, alice))
		detail, err := m.Route(ctx, r.ID, alice)
		require.NoError(t, err)
		assert.Equal(t, 2, detail.RunCount)
		assert.NotEmpty(t, detail.Track)

		require.NoError(t, m.DeleteWorkout(ctx, *detail.RepresentativeWorkoutID, alice))
		detail, err = m.Route(ctx, r.ID, alice)
		require.NoError(t, err, "the route outlives its representative until detection runs")
		assert.Nil(t, detail.RepresentativeWorkoutID)
		assert.Empty(t, detail.Track)

		require.NoError(t, m.DetectRoutes(ctx, alice))
		_, err = m.Route(ctx, r.ID, alice)
		assert.True(t, models.IsNotFound(err), "one run left is no longer a route: %v", err)
	})
}

func TestSportsAreScoredApart(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	upload := func(day int, sport typedef.Sport, secondsPerKm float64) *models.Workout {
		start := time.Date(2026, 11, day, 7, 0, 0, 0, time.UTC)
		content := testsupport.EncodeFITSport(t, start, []testsupport.Split{{Meters: 3000, Seconds: 3 * secondsPerKm}}, sport)
		af, _, err := m.AddActivityFile(ctx, bytes.NewReader(content), start, alice, repo)
		require.NoError(t, err)
		w, _, err := m.CreateWorkoutFromActivityFile(ctx, af.ID, alice, repo)
		require.NoError(t, err)
		return w
	}
	// All on the same path: two runs, two walks, two rides.
	run := upload(1, typedef.SportRunning, 300)
	upload(2, typedef.SportRunning, 290)
	walk := upload(3, typedef.SportWalking, 600)
	upload(4, typedef.SportWalking, 620)
	ride := upload(5, typedef.SportCycling, 120)
	upload(6, typedef.SportCycling, 110)

	assert.Equal(t, "running", run.Sport)
	assert.Equal(t, "cycling", ride.Sport)

	segments, err := m.WorkoutSegments(ctx, ride.ID, alice)
	require.NoError(t, err)
	assert.Empty(t, segments, "rides get no best efforts")

	runs, err := m.BestSegments(ctx, "1k", workouts.SportRunning, 10, alice)
	require.NoError(t, err)
	assert.Len(t, runs, 2, "the rides' faster kilometres don't count")
	walks, err := m.BestSegments(ctx, "1k", workouts.SportWalking, 10, alice)
	require.NoError(t, err)
	require.Len(t, walks, 2)
	assert.Equal(t, walk.ID, walks[0].WorkoutID)

	require.NoError(t, m.DetectRoutes(ctx, alice))
	routes, err := m.Routes(ctx, alice)
	require.NoError(t, err)
	require.Len(t, routes, 2, "one route each for runs and walks on the same path; rides get none")
	sports := []string{routes[0].Sport, routes[1].Sport}
	assert.ElementsMatch(t, []string{"running", "walking"}, sports)
	_, err = m.WorkoutRoute(ctx, ride.ID, alice)
	assert.ErrorIs(t, err, models.ErrNoRoute)
}
