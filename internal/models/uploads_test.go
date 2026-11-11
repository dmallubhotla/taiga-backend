package models_test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

func TestReuploadReturnsExistingFileAndWorkout(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice, bob := createUser(t, m, "alice@example.com"), createUser(t, m, "bob@example.com")
	fit := testsupport.EncodeFIT(t, day1, fourK)

	af, created, err := m.AddActivityFile(ctx, bytes.NewReader(fit), day1, alice, repo)
	require.NoError(t, err)
	assert.True(t, created)
	w, created, err := m.CreateWorkoutFromActivityFile(ctx, af.ID, alice, repo)
	require.NoError(t, err)
	assert.True(t, created)

	again, created, err := m.AddActivityFile(ctx, bytes.NewReader(fit), day2, alice, repo)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, af.ID, again.ID)
	wAgain, created, err := m.CreateWorkoutFromActivityFile(ctx, again.ID, alice, repo)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, w.ID, wAgain.ID)

	all, err := m.Workouts(ctx, models.WorkoutQuery{}, alice)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	// Dedupe is per user: the same bytes from someone else are their own file.
	bobs, created, err := m.AddActivityFile(ctx, bytes.NewReader(fit), day1, bob, repo)
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEqual(t, af.ID, bobs.ID)
}

func TestConcurrentReuploadsMakeOneFile(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	fit := testsupport.EncodeFIT(t, day1, fourK)

	const n = 8
	ids := make([]int32, n)
	createdCount := make([]bool, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			af, created, err := m.AddActivityFile(ctx, bytes.NewReader(fit), day1, alice, repo)
			if assert.NoError(t, err) {
				ids[i], createdCount[i] = af.ID, created
			}
		})
	}
	wg.Wait()

	for _, id := range ids {
		assert.Equal(t, ids[0], id)
	}
	var created int
	for _, c := range createdCount {
		if c {
			created++
		}
	}
	assert.Equal(t, 1, created, "exactly one upload creates the row")
	all, err := m.ActivityFiles(ctx, alice)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestCreateWorkoutRejectsUnusableFiles(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")

	for name, tc := range map[string]struct {
		content []byte
		want    error
	}{
		"junk":        {[]byte("definitely not a FIT file"), workouts.ErrUnreadableFile},
		"multi-sport": {testsupport.EncodeMultiSessionFIT(t, day1, fourK, 3), workouts.ErrUnsupportedActivity},
	} {
		t.Run(name, func(t *testing.T) {
			af, _, err := m.AddActivityFile(ctx, bytes.NewReader(tc.content), day1, alice, repo)
			require.NoError(t, err)

			_, _, err = m.CreateWorkoutFromActivityFile(ctx, af.ID, alice, repo)
			require.ErrorIs(t, err, tc.want)

			pending, err := m.ActivityFilesWithoutWorkout(ctx, alice)
			require.NoError(t, err)
			assert.Contains(t, activityFileIDs(pending), af.ID, "the failed file stays listed so it can be retried or deleted")
		})
	}
	all, err := m.Workouts(ctx, models.WorkoutQuery{}, alice)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestCreateWorkoutFromMissingFileIsNotFound(t *testing.T) {
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")

	_, _, err := m.CreateWorkoutFromActivityFile(context.Background(), 9999, alice, repo)
	assert.True(t, models.IsNotFound(err), "got %v", err)
}

func TestActivityFilesWithoutWorkout(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")

	w := uploadWorkout(t, m, repo, alice, day1, fourK)
	pending, _, err := m.AddActivityFile(ctx, bytes.NewReader(testsupport.EncodeFIT(t, day2, fourK)), day2, alice, repo)
	require.NoError(t, err)

	got, err := m.ActivityFilesWithoutWorkout(ctx, alice)
	require.NoError(t, err)
	assert.Equal(t, []int32{pending.ID}, activityFileIDs(got))
	assert.NotContains(t, activityFileIDs(got), *w.ActivityFileID)
}

func TestDeleteActivityFileInUse(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	w := uploadWorkout(t, m, repo, alice, day1, fourK)

	err := m.DeleteActivityFile(ctx, *w.ActivityFileID, alice)
	require.ErrorIs(t, err, models.ErrActivityFileInUse)

	require.NoError(t, m.DeleteWorkout(ctx, w.ID, alice))
	require.NoError(t, m.DeleteActivityFile(ctx, *w.ActivityFileID, alice))
}

func TestReprocessWorkout(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")

	// A workout from before processing was versioned: linked to its file, but with no summary or segments.
	af, _, err := m.AddActivityFile(ctx, bytes.NewReader(testsupport.EncodeFIT(t, day1, fourK)), day1, alice, repo)
	require.NoError(t, err)
	legacy := addWorkout(t, m, alice, &af.ID)
	assert.Empty(t, legacy.ProcessorVersion)
	_, err = m.WorkoutSeries(ctx, legacy.ID, alice)
	assert.ErrorIs(t, err, models.ErrNoSeries)

	reprocessed, err := m.ReprocessWorkout(ctx, legacy.ID, alice, repo)
	require.NoError(t, err)
	assert.Equal(t, workouts.ProcessorVersion, reprocessed.ProcessorVersion)
	require.NotNil(t, reprocessed.DistanceMiles)
	assert.InDelta(t, 4000/1609.344, *reprocessed.DistanceMiles, 0.01)
	segments, err := m.WorkoutSegments(ctx, legacy.ID, alice)
	require.NoError(t, err)
	byName := segmentsByName(segments)
	assert.Len(t, byName, 4, "1k, 1mi, 3k and 2mi fit in 4km")
	require.Contains(t, byName, "1k")
	assert.InDelta(t, 200, byName["1k"].DurationSeconds, 1)
	series, err := m.WorkoutSeries(ctx, legacy.ID, alice)
	require.NoError(t, err)
	assert.Contains(t, string(series), `"heart_rate":[150,`)

	// Reprocessing again replaces the segments rather than adding to them.
	_, err = m.ReprocessWorkout(ctx, legacy.ID, alice, repo)
	require.NoError(t, err)
	again, err := m.WorkoutSegments(ctx, legacy.ID, alice)
	require.NoError(t, err)
	assert.Len(t, again, len(segments))

	manual := addWorkout(t, m, alice, nil)
	_, err = m.ReprocessWorkout(ctx, manual.ID, alice, repo)
	assert.ErrorIs(t, err, models.ErrNoActivityFile)

	bob := createUser(t, m, "bob@example.com")
	_, err = m.ReprocessWorkout(ctx, legacy.ID, bob, repo)
	assert.True(t, models.IsNotFound(err), "bob can't reprocess alice's workout: %v", err)
}

func TestCreatedWorkoutsAreCurrent(t *testing.T) {
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	w := uploadWorkout(t, m, repo, alice, day1, fourK)
	assert.Equal(t, workouts.ProcessorVersion, w.ProcessorVersion)

	n, err := m.EnqueueStaleWorkouts(context.Background(), day1)
	require.NoError(t, err)
	assert.Zero(t, n)
}

func addWorkout(t *testing.T, m models.Model, user *tokens.UserToken, activityFileID *int32) *models.Workout {
	t.Helper()
	w, err := m.AddWorkout(context.Background(), &models.Workout{
		StartTime:      pgtype.Timestamptz{Time: day1, Valid: true},
		ActivityFileID: activityFileID,
	}, user)
	require.NoError(t, err)
	return w
}

func activityFileIDs(files []*models.ActivityFile) []int32 {
	ids := make([]int32, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}
