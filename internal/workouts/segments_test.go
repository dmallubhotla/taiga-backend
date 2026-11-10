package workouts

import (
	"math"
	"testing"
	"time"
)

// runFromPaces builds a run of equal-length splits at the given paces (seconds per meter), ten records per split.
func runFromPaces(secondsPerMeter []float64, metersEach float64) *Run {
	run := &Run{}
	var meters float64
	var elapsed time.Duration
	run.Records = append(run.Records, &RecordData{Distance: &Distance{0}})
	for _, pace := range secondsPerMeter {
		step := metersEach / 10
		for range 10 {
			meters += step
			elapsed += time.Duration(pace * step * float64(time.Second))
			run.Records = append(run.Records, &RecordData{Distance: &Distance{meters}, DurationFromStart: elapsed})
		}
	}
	return run
}

func TestFastestSegmentsFindsFastestWindow(t *testing.T) {
	// Three 1000m splits: slow, fast, slow.
	run := runFromPaces([]float64{0.36, 0.24, 0.36}, 1000)

	got := FastestSegments(run, []NamedDistance{{"1k", 1000}})

	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].Name != "1k" {
		t.Errorf("Name = %q, want 1k", got[0].Name)
	}
	if want := 240 * time.Second; got[0].Duration != want {
		t.Errorf("Duration = %v, want %v", got[0].Duration, want)
	}
	if got[0].StartOffset() != 360*time.Second || got[0].EndOffset() != 600*time.Second {
		t.Errorf("window = %v..%v, want 6m0s..10m0s", got[0].StartOffset(), got[0].EndOffset())
	}
}

func TestFastestSegmentsCoversFullTarget(t *testing.T) {
	// Uneven spacing: the best window must span >= 1000m, never just under it.
	run := runFromPaces([]float64{0.3, 0.3, 0.3}, 700)

	got := FastestSegments(run, []NamedDistance{{"1k", 1000}})

	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	covered := run.Records[got[0].End].Distance.Meters - run.Records[got[0].Start].Distance.Meters
	if covered < 1000 {
		t.Errorf("covered %vm, want >= 1000m", covered)
	}
}

func TestFastestSegmentsOmitsUnreachedTargets(t *testing.T) {
	run := runFromPaces([]float64{0.3}, 3000)

	got := FastestSegments(run, StandardDistances)

	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}
	if len(names) != 2 || names[0] != "1k" || names[1] != "1mi" {
		t.Errorf("names = %v, want [1k 1mi]", names)
	}
}

func TestFastestSegmentsSkipsRecordsWithoutDistance(t *testing.T) {
	run := runFromPaces([]float64{0.3, 0.3}, 1000)
	run.Records[0].Distance.Meters = math.NaN()
	run.Records[5].Distance = nil

	got := FastestSegments(run, []NamedDistance{{"1k", 1000}})

	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if want := 300 * time.Second; got[0].Duration != want {
		t.Errorf("Duration = %v, want %v", got[0].Duration, want)
	}
}

func TestStandardDistance(t *testing.T) {
	if d, ok := StandardDistance("5k"); !ok || d.Meters != 5000 {
		t.Errorf("StandardDistance(5k) = %v, %v", d, ok)
	}
	if _, ok := StandardDistance("3k"); ok {
		t.Error("StandardDistance(3k) should not exist")
	}
}
