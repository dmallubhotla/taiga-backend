package models_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

func TestWorkoutWindows(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	bob := createUser(t, m, "bob@example.com")
	day := func(y int, mo time.Month, d int) time.Time { return time.Date(y, mo, d, 7, 0, 0, 0, time.UTC) }
	// Fastest 1k in 2025; 2026 has two slower runs and one that doesn't count.
	old := uploadWorkout(t, m, repo, alice, day(2025, 6, 1), []testsupport.Split{{Meters: 2000, Seconds: 500}})
	jan := uploadWorkout(t, m, repo, alice, day(2026, 1, 1), []testsupport.Split{{Meters: 2000, Seconds: 600}})
	mar := uploadWorkout(t, m, repo, alice, day(2026, 3, 1), []testsupport.Split{{Meters: 2000, Seconds: 700}})
	excluded := uploadWorkout(t, m, repo, alice, day(2026, 5, 1), []testsupport.Split{{Meters: 2000, Seconds: 550}})
	_, err := m.CorrectWorkout(ctx, excluded.ID, models.Corrections{ExcludedReason: "ran with a friend"}, alice, repo)
	require.NoError(t, err)
	uploadWorkout(t, m, repo, bob, day(2026, 2, 1), []testsupport.Split{{Meters: 2000, Seconds: 600}})

	year := models.Window{From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}
	ids := func(ws []*models.Workout) []int32 {
		out := make([]int32, 0, len(ws))
		for _, w := range ws {
			out = append(out, w.ID)
		}
		return out
	}

	t.Run("list", func(t *testing.T) {
		all, err := m.Workouts(ctx, models.WorkoutQuery{}, alice)
		require.NoError(t, err)
		assert.Equal(t, []int32{excluded.ID, mar.ID, jan.ID, old.ID}, ids(all), "newest first, other users' left out")

		inYear, err := m.Workouts(ctx, models.WorkoutQuery{Window: year}, alice)
		require.NoError(t, err)
		assert.Equal(t, []int32{excluded.ID, mar.ID, jan.ID}, ids(inYear), "from is inclusive, and excluded workouts are still listed")

		before, err := m.Workouts(ctx, models.WorkoutQuery{Window: models.Window{To: year.From}}, alice)
		require.NoError(t, err)
		assert.Equal(t, []int32{old.ID}, ids(before), "to is exclusive")

		page, err := m.Workouts(ctx, models.WorkoutQuery{Window: year, Limit: 2, Offset: 1}, alice)
		require.NoError(t, err)
		assert.Equal(t, []int32{mar.ID, jan.ID}, ids(page))
	})

	t.Run("a local midnight bound", func(t *testing.T) {
		// 2026-01-01 00:00 in UTC-6 is 06:00 UTC, before the 07:00 UTC run, so it's in the window.
		chicago := time.FixedZone("CST", -6*60*60)
		w := models.Window{From: time.Date(2026, 1, 1, 0, 0, 0, 0, chicago), To: time.Date(2026, 1, 2, 0, 0, 0, 0, chicago)}
		got, err := m.Workouts(ctx, models.WorkoutQuery{Window: w}, alice)
		require.NoError(t, err)
		assert.Equal(t, []int32{jan.ID}, ids(got))
	})

	t.Run("totals", func(t *testing.T) {
		totals, err := m.WorkoutTotals(ctx, year, alice)
		require.NoError(t, err)
		require.Len(t, totals.Sports, 1)
		running := totals.Sports[0]
		assert.Equal(t, workouts.SportRunning, running.Sport)
		assert.Equal(t, int64(2), running.Workouts, "the excluded run doesn't count")
		assert.InDelta(t, 2*2000/1609.344, running.DistanceMiles, 0.01)
		assert.InDelta(t, 1300, running.TimeSeconds, 1)
		require.NotNil(t, totals.FirstStartTime)
		assert.True(t, totals.FirstStartTime.Equal(day(2026, 1, 1)))

		allTime, err := m.WorkoutTotals(ctx, models.Window{}, alice)
		require.NoError(t, err)
		require.NotNil(t, allTime.FirstStartTime)
		assert.True(t, allTime.FirstStartTime.Equal(day(2025, 6, 1)), "the earliest workout tells a client which years to offer")

		none, err := m.WorkoutTotals(ctx, models.Window{To: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, alice)
		require.NoError(t, err)
		assert.Empty(t, none.Sports)
		assert.Nil(t, none.FirstStartTime)
	})

	t.Run("best efforts", func(t *testing.T) {
		allTime, err := m.BestSegments(ctx, "1k", workouts.SportRunning, 10, models.Window{}, alice)
		require.NoError(t, err)
		require.Len(t, allTime, 3)
		assert.Equal(t, old.ID, allTime[0].WorkoutID)

		thisYear, err := m.BestSegments(ctx, "1k", workouts.SportRunning, 10, year, alice)
		require.NoError(t, err)
		require.Len(t, thisYear, 2)
		assert.Equal(t, jan.ID, thisYear[0].WorkoutID)
		assert.Equal(t, mar.ID, thisYear[1].WorkoutID)
	})
}
