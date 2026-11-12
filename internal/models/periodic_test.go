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

func TestPeriodicTotals(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	chicago, err := time.LoadLocation("America/Chicago")
	require.NoError(t, err)
	at := func(mo time.Month, d, hour int) time.Time { return time.Date(2026, mo, d, hour, 0, 0, 0, chicago) }
	mile := []testsupport.Split{{Meters: 1609.344, Seconds: 480}}

	// Monday 2 March, and Sunday 8 March at 9pm: 3am Monday in UTC, but the same local week.
	uploadWorkout(t, m, repo, alice, at(time.March, 2, 7), mile)
	uploadWorkout(t, m, repo, alice, at(time.March, 8, 21), mile)
	// Nothing the week of 9 March; then 31 March, a Tuesday, whose week runs into April.
	uploadWorkout(t, m, repo, alice, at(time.March, 31, 7), mile)
	excluded := uploadWorkout(t, m, repo, alice, at(time.March, 17, 7), mile)
	_, err = m.CorrectWorkout(ctx, excluded.ID, models.Corrections{ExcludedReason: "ran with a friend"}, alice, repo)
	require.NoError(t, err)

	starts := func(ps []*models.PeriodTotals) []time.Time {
		out := make([]time.Time, 0, len(ps))
		for _, p := range ps {
			out = append(out, p.Start)
		}
		return out
	}
	miles := func(ps []*models.PeriodTotals) []float64 {
		out := make([]float64, 0, len(ps))
		for _, p := range ps {
			total := 0.0
			for _, s := range p.Sports {
				total += s.DistanceMiles
			}
			out = append(out, total)
		}
		return out
	}

	t.Run("weeks between the first and last workout", func(t *testing.T) {
		weeks, err := m.PeriodicTotals(ctx, models.EveryWeek, models.Window{}, chicago, alice)
		require.NoError(t, err)
		assert.Equal(t, []time.Time{at(time.March, 2, 0), at(time.March, 9, 0), at(time.March, 16, 0), at(time.March, 23, 0), at(time.March, 30, 0)}, starts(weeks))
		got := miles(weeks)
		for i, want := range []float64{2, 0, 0, 0, 1} {
			assert.InDelta(t, want, got[i], 0.01, "week %d; the excluded run doesn't count", i)
		}
		require.Len(t, weeks[0].Sports, 1)
		assert.Equal(t, workouts.SportRunning, weeks[0].Sports[0].Sport)
		assert.Equal(t, int64(2), weeks[0].Sports[0].Workouts)
		assert.InDelta(t, 960, weeks[0].Sports[0].TimeSeconds, 1)
	})

	t.Run("in UTC the Sunday night run is a week later", func(t *testing.T) {
		weeks, err := m.PeriodicTotals(ctx, models.EveryWeek, models.Window{}, time.UTC, alice)
		require.NoError(t, err)
		assert.InDeltaSlice(t, []float64{1, 1, 0, 0, 1}, miles(weeks), 0.01)
	})

	t.Run("months across a bounded window", func(t *testing.T) {
		window := models.Window{From: at(time.January, 1, 0), To: at(time.May, 1, 0)}
		months, err := m.PeriodicTotals(ctx, models.EveryMonth, window, chicago, alice)
		require.NoError(t, err)
		assert.Equal(t, []time.Time{at(time.January, 1, 0), at(time.February, 1, 0), at(time.March, 1, 0), at(time.April, 1, 0)}, starts(months), "the window's empty months are listed too, and to is exclusive")
		assert.InDeltaSlice(t, []float64{0, 0, 3, 0}, miles(months), 0.01)
	})

	t.Run("nothing to list", func(t *testing.T) {
		none, err := m.PeriodicTotals(ctx, models.EveryMonth, models.Window{To: at(time.January, 1, 0)}, chicago, alice)
		require.NoError(t, err)
		assert.Empty(t, none)
	})

	t.Run("too many periods", func(t *testing.T) {
		window := models.Window{From: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)}
		_, err := m.PeriodicTotals(ctx, models.EveryWeek, window, chicago, alice)
		assert.ErrorIs(t, err, models.ErrTooManyPeriods)
	})
}

func TestYearlyBests(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	bob := createUser(t, m, "bob@example.com")
	chicago, err := time.LoadLocation("America/Chicago")
	require.NoError(t, err)

	slow := uploadWorkout(t, m, repo, alice, time.Date(2025, 3, 1, 7, 0, 0, 0, time.UTC), []testsupport.Split{{Meters: 2000, Seconds: 700}})
	fast := uploadWorkout(t, m, repo, alice, time.Date(2025, 6, 1, 7, 0, 0, 0, time.UTC), []testsupport.Split{{Meters: 5000, Seconds: 1500}})
	// 2026-01-01 03:00 UTC is still New Year's Eve in Chicago.
	eve := uploadWorkout(t, m, repo, alice, time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC), []testsupport.Split{{Meters: 2000, Seconds: 500}})
	recent := uploadWorkout(t, m, repo, alice, time.Date(2026, 2, 1, 7, 0, 0, 0, time.UTC), []testsupport.Split{{Meters: 2000, Seconds: 600}})
	uploadWorkout(t, m, repo, bob, time.Date(2024, 2, 1, 7, 0, 0, 0, time.UTC), []testsupport.Split{{Meters: 2000, Seconds: 400}})

	years, err := m.YearlyBests(ctx, workouts.SportRunning, chicago, alice)
	require.NoError(t, err)
	require.Len(t, years, 2, "bob's year isn't alice's")
	assert.Equal(t, 2026, years[0].Year, "newest first")
	assert.Equal(t, recent.ID, years[0].Efforts["1k"].WorkoutID)
	assert.NotContains(t, years[0].Efforts, "5k")

	assert.Equal(t, 2025, years[1].Year)
	assert.Equal(t, eve.ID, years[1].Efforts["1k"].WorkoutID, "New Year's Eve in Chicago is 2025's")
	assert.Equal(t, fast.ID, years[1].Efforts["5k"].WorkoutID)
	assert.NotEqual(t, slow.ID, years[1].Efforts["1k"].WorkoutID)

	inUTC, err := m.YearlyBests(ctx, workouts.SportRunning, time.UTC, alice)
	require.NoError(t, err)
	assert.Equal(t, eve.ID, inUTC[0].Efforts["1k"].WorkoutID, "in UTC it's 2026's")

	walks, err := m.YearlyBests(ctx, workouts.SportWalking, chicago, alice)
	require.NoError(t, err)
	assert.Empty(t, walks)
}
