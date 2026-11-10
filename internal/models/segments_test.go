package models_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

func createUser(t *testing.T, m models.Model, email string) *tokens.UserToken {
	t.Helper()
	resp, err := m.CreateUser(context.Background(), &models.CreateUserRequest{Email: email, DisplayName: email, Password: "password123"})
	require.NoError(t, err)
	return &tokens.UserToken{ID: resp.Id, Email: email}
}

func uploadWorkout(t *testing.T, m models.Model, repo filerepo.FileRepo, user *tokens.UserToken, start time.Time, splits []testsupport.Split) *models.Workout {
	t.Helper()
	ctx := context.Background()
	af, _, err := m.AddActivityFile(ctx, bytes.NewReader(testsupport.EncodeFIT(t, start, splits)), start, user, repo)
	require.NoError(t, err)
	w, _, err := m.CreateWorkoutFromActivityFile(ctx, af.ID, user, repo)
	require.NoError(t, err)
	return w
}

func segmentsByName(segments []*models.WorkoutSegment) map[string]*models.WorkoutSegment {
	out := map[string]*models.WorkoutSegment{}
	for _, s := range segments {
		out[s.Name] = s
	}
	return out
}

var day1 = time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
var day2 = day1.AddDate(0, 0, 1)

// 4km: 1km at 5:00/km, a fast 1km at 3:20/km, 2km at 5:00/km.
var fourK = []testsupport.Split{{Meters: 1000, Seconds: 300}, {Meters: 1000, Seconds: 200}, {Meters: 2000, Seconds: 600}}

func TestCreateWorkoutStoresSegments(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	user := createUser(t, m, "a@example.com")

	w := uploadWorkout(t, m, repo, user, day1, fourK)
	require.NotNil(t, w.DistanceMiles)
	assert.InDelta(t, 4000/1609.344, *w.DistanceMiles, 1e-3)

	segments, err := m.WorkoutSegments(ctx, w.ID, user)
	require.NoError(t, err)
	byName := segmentsByName(segments)
	assert.Len(t, segments, 4, "4km covers 1k, 1mi, 3k and 2mi only")

	oneK := byName["1k"]
	require.NotNil(t, oneK)
	assert.Equal(t, 200.0, oneK.DurationSeconds)
	assert.Equal(t, 300.0, oneK.StartOffsetSeconds)
	assert.Equal(t, 500.0, oneK.EndOffsetSeconds)

	// The fast km plus the quickest way to cover the other 609.344m (at 5:00/km), measured to the next 10m record.
	mile := byName["1mi"]
	require.NotNil(t, mile)
	assert.Equal(t, 1609.344, mile.DistanceMeters)
	assert.InDelta(t, 200+610*0.3, mile.DurationSeconds, 1e-9)
}

func TestBestSegmentsAcrossWorkouts(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	user := createUser(t, m, "a@example.com")
	other := createUser(t, m, "b@example.com")

	slow := uploadWorkout(t, m, repo, user, day1, []testsupport.Split{{Meters: 2000, Seconds: 800}})
	fast := uploadWorkout(t, m, repo, user, day2, fourK)
	uploadWorkout(t, m, repo, other, day2, []testsupport.Split{{Meters: 1000, Seconds: 100}})

	efforts, err := m.BestSegments(ctx, "1k", 10, user)
	require.NoError(t, err)
	require.Len(t, efforts, 2, "other users' efforts are excluded")
	assert.Equal(t, fast.ID, efforts[0].WorkoutID)
	assert.Equal(t, 200.0, efforts[0].DurationSeconds)
	assert.True(t, efforts[0].WorkoutStartTime.Time.Equal(day2))
	assert.Equal(t, slow.ID, efforts[1].WorkoutID)
	assert.Equal(t, 400.0, efforts[1].DurationSeconds)

	limited, err := m.BestSegments(ctx, "1k", 1, user)
	require.NoError(t, err)
	assert.Len(t, limited, 1)
}

func TestCompareWorkouts(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	user := createUser(t, m, "a@example.com")

	short := uploadWorkout(t, m, repo, user, day1, []testsupport.Split{{Meters: 1200, Seconds: 480}})
	long := uploadWorkout(t, m, repo, user, day2, fourK)

	comparisons, err := m.CompareWorkouts(ctx, []int32{long.ID, short.ID}, user)
	require.NoError(t, err)
	require.Len(t, comparisons, 2)
	assert.Equal(t, long.ID, comparisons[0].Workout.ID, "order follows the request")
	assert.Contains(t, comparisons[0].Segments, "1mi")
	assert.Contains(t, comparisons[1].Segments, "1k")
	assert.NotContains(t, comparisons[1].Segments, "1mi")

	_, err = m.CompareWorkouts(ctx, []int32{long.ID, 9999}, user)
	assert.Error(t, err)
}

func TestSegmentsAreScopedToOwner(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	owner := createUser(t, m, "a@example.com")
	intruder := createUser(t, m, "b@example.com")
	w := uploadWorkout(t, m, repo, owner, day1, fourK)

	_, err := m.WorkoutSegments(ctx, w.ID, intruder)
	assert.Error(t, err)
	_, err = m.CompareWorkouts(ctx, []int32{w.ID}, intruder)
	assert.Error(t, err)
}

func TestDeleteWorkoutRemovesSegments(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	user := createUser(t, m, "a@example.com")
	keep := uploadWorkout(t, m, repo, user, day1, fourK)
	drop := uploadWorkout(t, m, repo, user, day2, fourK)

	require.NoError(t, m.DeleteWorkout(ctx, drop.ID, user))

	efforts, err := m.BestSegments(ctx, "1k", 10, user)
	require.NoError(t, err)
	require.Len(t, efforts, 1)
	assert.Equal(t, keep.ID, efforts[0].WorkoutID)
}
