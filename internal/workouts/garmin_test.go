package workouts

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
)

func TestSummariseUsesElapsedTime(t *testing.T) {
	start := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	// Timestamp == StartTime mirrors files where the wall-clock span is zero.
	sesh := mesgdef.NewSession(nil).
		SetStartTime(start).
		SetTimestamp(start).
		SetTotalDistanceScaled(1609.344 * 5).
		SetTotalElapsedTimeScaled(40 * 60)

	s := summarise(sesh)

	if math.Abs(s.DistanceMiles-5) > 1e-4 {
		t.Errorf("DistanceMiles = %v, want 5", s.DistanceMiles)
	}
	if math.Abs(s.PaceMinPerMile-8) > 1e-4 {
		t.Errorf("PaceMinPerMile = %v, want 8", s.PaceMinPerMile)
	}
	if math.Abs(s.SpeedMph-7.5) > 1e-4 {
		t.Errorf("SpeedMph = %v, want 7.5", s.SpeedMph)
	}
}

func TestSummariseFallsBackToWallClock(t *testing.T) {
	start := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	sesh := mesgdef.NewSession(nil).
		SetStartTime(start).
		SetTimestamp(start.Add(30 * time.Minute)).
		SetTotalDistanceScaled(1609.344 * 3)

	s := summarise(sesh)

	if math.Abs(s.PaceMinPerMile-10) > 1e-4 {
		t.Errorf("PaceMinPerMile = %v, want 10", s.PaceMinPerMile)
	}
}

func TestReadActivityNeedsExactlyOneSession(t *testing.T) {
	start := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	sesh := mesgdef.NewSession(nil).SetStartTime(start).SetTimestamp(start.Add(time.Hour))

	for _, n := range []int{0, 2} {
		activity := filedef.NewActivity()
		for range n {
			activity.Sessions = append(activity.Sessions, sesh)
		}
		if _, err := ReadActivity(activity); !errors.Is(err, ErrUnsupportedActivity) {
			t.Errorf("%d sessions: err = %v, want ErrUnsupportedActivity", n, err)
		}
	}
}
