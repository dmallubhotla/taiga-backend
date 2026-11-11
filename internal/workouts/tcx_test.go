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

var tcxStart = time.Date(2022, 5, 1, 14, 0, 0, 0, time.UTC)

func readTCX(t *testing.T, x *testsupport.TCX) *workouts.Run {
	t.Helper()
	run, err := workouts.ReadActivityFile(bytes.NewReader(x.Encode(t)))
	require.NoError(t, err)
	return run
}

func TestReadTCXRun(t *testing.T) {
	splits := []testsupport.Split{{Meters: 1000, Seconds: 300}, {Meters: 2000, Seconds: 660}}
	run := readTCX(t, testsupport.NewTCX(tcxStart, splits, "Running"))

	s := run.Summary
	assert.Equal(t, "running", s.Sport)
	assert.Empty(t, s.ExcludedReason)
	assert.True(t, s.Scored())
	assert.Equal(t, "Google Fit", s.Source)
	assert.Equal(t, tcxStart, s.StartTime)
	assert.InDelta(t, 960, s.TimeSeconds, 1e-6)
	assert.InDelta(t, 3000/1609.344, s.DistanceMiles, 1e-6)

	require.Len(t, run.Records, 301)
	assert.InDelta(t, 3000, run.Records[300].Distance.Meters, 1e-6)
	assert.InDelta(t, testsupport.StartLatitude, run.Records[0].Latitude, 1e-9)
	segments := workouts.FastestSegments(run, workouts.StandardDistances)
	require.NotEmpty(t, segments)
	assert.InDelta(t, 300, segments[0].Duration.Seconds(), 1e-6)
	assert.NotNil(t, workouts.BuildTrack(run))
}

// Google Fit splits a session into activities that each restart their distance.
func TestReadTCXLaysActivitiesEndToEnd(t *testing.T) {
	splits := []testsupport.Split{{Meters: 3000, Seconds: 900}}
	run := readTCX(t, testsupport.NewTCX(tcxStart, splits, "Running", "Running", "Running"))
	assert.Empty(t, run.Summary.ExcludedReason)
	assert.InDelta(t, 3000/1609.344, run.Summary.DistanceMiles, 1e-6)
	last := 0.0
	for _, rec := range run.Records {
		assert.GreaterOrEqual(t, rec.Distance.Meters, last)
		last = rec.Distance.Meters
	}
	assert.InDelta(t, 3000, last, 1e-6)
}

func TestReadTCXSports(t *testing.T) {
	splits := []testsupport.Split{{Meters: 1000, Seconds: 300}}
	for sport, want := range map[string][2]string{
		"Walking":           {"walking", ""},
		"Running.treadmill": {"running", "treadmill"},
		"Biking":            {"cycling", ""},
		"Strength_training": {"training", "strength_training"},
		"Kitesurfing":       {"generic", ""},
	} {
		s := readTCX(t, testsupport.NewTCX(tcxStart, splits, sport)).Summary
		assert.Equal(t, want, [2]string{s.Sport, s.SubSport}, sport)
		assert.Empty(t, s.ExcludedReason, sport)
	}
}

// Sessions Google Fit detected on its own rather than tracked keep their totals, lose their points, and never count.
func TestReadTCXExcludesAutoDetectedSessions(t *testing.T) {
	splits := []testsupport.Split{{Meters: 3000, Seconds: 900}}
	for name, tc := range map[string]struct {
		session func() *testsupport.TCX
		reason  string
	}{
		"walk and run mixed": {func() *testsupport.TCX {
			return testsupport.NewTCX(tcxStart, splits, "Running", "Walking")
		}, workouts.ExcludedMixedSports},
		"a fix every few hundred meters": {func() *testsupport.TCX {
			x := testsupport.NewTCX(tcxStart, splits, "Running")
			for i := range x.Points {
				x.Points[i].NoPosition = i%30 != 0
			}
			return x
		}, workouts.ExcludedSparseGPS},
		"time runs backwards": {func() *testsupport.TCX {
			x := testsupport.NewTCX(tcxStart, splits, "Running")
			x.Points[5].Time = x.Points[5].Time.Add(-time.Hour)
			return x
		}, workouts.ExcludedTimeBackwards},
	} {
		t.Run(name, func(t *testing.T) {
			run := readTCX(t, tc.session())
			assert.Equal(t, tc.reason, run.Summary.ExcludedReason)
			assert.False(t, run.Summary.Scored())
			assert.Empty(t, run.Records)
			assert.InDelta(t, 3000/1609.344, run.Summary.DistanceMiles, 1e-6, "the totals stay")
		})
	}
}

func TestReadTCXTreadmillIsNotFlaggedForGPS(t *testing.T) {
	x := testsupport.NewTCX(tcxStart, []testsupport.Split{{Meters: 3000, Seconds: 900}}, "Running.treadmill")
	for i := range x.Points {
		x.Points[i].NoPosition = true
	}
	s := readTCX(t, x).Summary
	assert.Empty(t, s.ExcludedReason)
	assert.False(t, s.Scored(), "indoor runs never count anyway")
}

func TestReadActivityFileRejectsOtherXML(t *testing.T) {
	for name, content := range map[string]string{
		"other XML":  `<?xml version="1.0"?><gpx><trk/></gpx>`,
		"no laps":    `<TrainingCenterDatabase><Activities><Activity Sport="Running"/></Activities></TrainingCenterDatabase>`,
		"truncated":  `<TrainingCenterDatabase><Activities><Activity Sport="Running"><Lap StartTime="2022-05-01T14:00:00Z">`,
		"not XML at": `<<<`,
	} {
		_, err := workouts.ReadActivityFile(strings.NewReader(content))
		assert.Error(t, err, name)
	}
}
