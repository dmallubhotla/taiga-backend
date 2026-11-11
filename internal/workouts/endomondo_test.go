package workouts_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

var endomondoStart = time.Date(2018, 6, 1, 22, 0, 0, 0, time.UTC)

func readEndomondo(t *testing.T, e *testsupport.Endomondo) *workouts.Run {
	t.Helper()
	run, err := workouts.ReadActivityFile(bytes.NewReader(e.Encode(t)))
	require.NoError(t, err)
	return run
}

func TestReadEndomondoRun(t *testing.T) {
	splits := []testsupport.Split{{Meters: 1000, Seconds: 300}, {Meters: 2000, Seconds: 660}}
	run := readEndomondo(t, testsupport.NewEndomondo(endomondoStart, splits, "RUNNING"))

	s := run.Summary
	assert.Equal(t, "running", s.Sport)
	assert.Empty(t, s.SubSport)
	assert.Empty(t, s.ExcludedReason)
	assert.True(t, s.Scored())
	assert.Equal(t, endomondoStart, s.StartTime)
	assert.InDelta(t, 960, s.TimeSeconds, 1)
	assert.InDelta(t, 3000/1609.344, s.DistanceMiles, 1e-6)

	require.Len(t, run.Records, 301)
	last := run.Records[300]
	assert.InDelta(t, 3000, last.Distance.Meters, 1e-6)
	assert.InDelta(t, 960, last.DurationFromStart.Seconds(), 1)
	assert.InDelta(t, testsupport.StartLatitude, run.Records[0].Latitude, 1e-9)
	assert.Equal(t, float64(testsupport.RecordHeartRate), last.HeartRate)

	segments := workouts.FastestSegments(run, workouts.StandardDistances)
	require.NotEmpty(t, segments)
	assert.Equal(t, "1k", segments[0].Name)
	assert.InDelta(t, 300, segments[0].Duration.Seconds(), 1e-6)
	assert.NotNil(t, workouts.BuildTrack(run))
}

func TestReadEndomondoSports(t *testing.T) {
	splits := []testsupport.Split{{Meters: 1000, Seconds: 300}}
	for sport, want := range map[string][2]string{
		"WALKING":           {"walking", ""},
		"TREADMILL_RUNNING": {"running", "treadmill"},
		"SPINNING":          {"cycling", "indoor_cycling"},
		"WEIGHT_TRAINING":   {"training", "strength_training"},
		"KITE_SURFING":      {"generic", ""},
	} {
		s := readEndomondo(t, testsupport.NewEndomondo(endomondoStart, splits, sport)).Summary
		assert.Equal(t, want, [2]string{s.Sport, s.SubSport}, sport)
	}
}

// Files that can't be trusted keep their stated totals, lose their points, and never count.
func TestReadEndomondoExcludesBadFiles(t *testing.T) {
	splits := []testsupport.Split{{Meters: 3000, Seconds: 900}}
	for name, tc := range map[string]struct {
		spoil  func(e *testsupport.Endomondo)
		reason string
	}{
		"hand-entered": {func(e *testsupport.Endomondo) {
			e.Source, e.Points = "INPUT_MANUAL", nil
		}, workouts.ExcludedHandEntered},
		"no distance": {func(e *testsupport.Endomondo) {
			e.DistanceKm, e.Points = 0, e.Points[:3]
			for i := range e.Points {
				e.Points[i].DistanceKm = 0
			}
		}, workouts.ExcludedNoDistance},
		"GPS lost partway": {func(e *testsupport.Endomondo) {
			e.Points = e.Points[:20]
		}, workouts.ExcludedPointsDisagree},
		"points from before the start": {func(e *testsupport.Endomondo) {
			for i := range e.Points {
				e.Points[i].Time = e.Points[i].Time.Add(-time.Hour)
			}
		}, workouts.ExcludedPointsDisagree},
		"time runs backwards": {func(e *testsupport.Endomondo) {
			e.Points[0].Time = e.Points[0].Time.Add(time.Hour)
		}, workouts.ExcludedTimeBackwards},
	} {
		t.Run(name, func(t *testing.T) {
			e := testsupport.NewEndomondo(endomondoStart, splits, "RUNNING")
			tc.spoil(e)
			run := readEndomondo(t, e)
			assert.Equal(t, tc.reason, run.Summary.ExcludedReason)
			assert.False(t, run.Summary.Scored())
			assert.Empty(t, run.Records)
			assert.InDelta(t, e.DistanceKm*1000/1609.344, run.Summary.DistanceMiles, 1e-6, "the stated total stays")
		})
	}
}

func TestReadEndomondoPointsMayRunPastTheEnd(t *testing.T) {
	// end_time is the start plus moving time, so a paused run's points carry on after it.
	e := testsupport.NewEndomondo(endomondoStart, []testsupport.Split{{Meters: 3000, Seconds: 900}}, "RUNNING")
	e.Duration -= 3 * time.Minute
	run := readEndomondo(t, e)
	assert.Empty(t, run.Summary.ExcludedReason)
	assert.NotEmpty(t, run.Records)
}

func TestReadActivityFileTellsFormatsApart(t *testing.T) {
	start := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	splits := []testsupport.Split{{Meters: 1000, Seconds: 300}}
	fit, err := workouts.ReadActivityFile(bytes.NewReader(testsupport.EncodeFIT(t, start, splits)))
	require.NoError(t, err)
	assert.Equal(t, start, fit.Summary.StartTime)

	for name, content := range map[string]string{
		"junk":                 "definitely not an activity file",
		"other JSON":           `{"type": "FeatureCollection"}`,
		"JSON without a start": `[{"sport": "RUNNING"}]`,
		"truncated":            `[{"sport": "RUNNING"}, {"start_ti`,
	} {
		_, err := workouts.ReadActivityFile(strings.NewReader(content))
		assert.ErrorIs(t, err, workouts.ErrUnreadableFile, name)
	}
}
