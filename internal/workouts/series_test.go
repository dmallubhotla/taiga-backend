package workouts

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func recordsRun(n int, withHR bool) *Run {
	run := &Run{}
	for i := range n {
		hr := math.NaN()
		if withHR {
			hr = 140
		}
		run.Records = append(run.Records, &RecordData{
			Distance:          &Distance{float64(i) * 3},
			DurationFromStart: time.Duration(i) * time.Second,
			SpeedMeterPerSec:  3,
			AltitudeMeters:    math.NaN(),
			HeartRate:         hr,
			Latitude:          math.NaN(),
			Longitude:         math.NaN(),
		})
	}
	return run
}

func TestBuildSeriesDownsamplesKeepingEnds(t *testing.T) {
	for _, n := range []int{1, 10, 2000, 2001, 3600, 14_401} {
		s := BuildSeries(recordsRun(n, true), MaxSeriesPoints)
		if len(s.TimeSeconds) > MaxSeriesPoints {
			t.Errorf("n=%d: %d points, want at most %d", n, len(s.TimeSeconds), MaxSeriesPoints)
		}
		if n <= MaxSeriesPoints && len(s.TimeSeconds) != n {
			t.Errorf("n=%d: %d points, want all of them", n, len(s.TimeSeconds))
		}
		if s.TimeSeconds[0] != 0 || s.TimeSeconds[len(s.TimeSeconds)-1] != float64(n-1) {
			t.Errorf("n=%d: spans %v..%v, want 0..%d", n, s.TimeSeconds[0], s.TimeSeconds[len(s.TimeSeconds)-1], n-1)
		}
		if len(s.DistanceMeters) != len(s.TimeSeconds) || len(s.HeartRate) != len(s.TimeSeconds) {
			t.Errorf("n=%d: columns differ in length", n)
		}
	}
}

func TestBuildSeriesOmitsUnrecordedColumns(t *testing.T) {
	run := recordsRun(3, false)
	run.Records[1].AltitudeMeters = 250
	b, err := json.Marshal(BuildSeries(run, MaxSeriesPoints))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, absent := range []string{"heart_rate", "lat", "lon"} {
		if strings.Contains(got, absent) {
			t.Errorf("%s should be omitted when never recorded: %s", absent, got)
		}
	}
	if !strings.Contains(got, `"altitude_m":[null,250,null]`) {
		t.Errorf("a partly recorded column should keep nulls in place: %s", got)
	}
	if !strings.Contains(got, `"distance_m":[0,3,6]`) {
		t.Errorf("distance column: %s", got)
	}
}
