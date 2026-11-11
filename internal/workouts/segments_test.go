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
	run := runFromPaces([]float64{0.3}, 2500)

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
	if _, ok := StandardDistance("4k"); ok {
		t.Error("StandardDistance(4k) should not exist")
	}
}

func TestFarthestEffortsFindsFarthestWindow(t *testing.T) {
	// Three 1000m splits: slow, fast, slow.
	run := runFromPaces([]float64{0.36, 0.24, 0.36}, 1000)

	got := FarthestEfforts(run, []NamedDuration{{"4min", 4 * time.Minute}, {"1h", time.Hour}})

	if len(got) != 1 {
		t.Fatalf("got %d efforts, want 1 (the run is shorter than an hour)", len(got))
	}
	if got[0].Name != "4min" || got[0].Kind != KindDuration {
		t.Errorf("Name, Kind = %q, %q", got[0].Name, got[0].Kind)
	}
	if math.Abs(got[0].Distance.Meters-1000) > 1e-9 {
		t.Errorf("Distance = %vm, want 1000m", got[0].Distance.Meters)
	}
	if got[0].StartOffset() != 360*time.Second || got[0].EndOffset() != 600*time.Second {
		t.Errorf("window = %v..%v, want 6m0s..10m0s", got[0].StartOffset(), got[0].EndOffset())
	}
}

func TestFarthestEffortsLastsFullTarget(t *testing.T) {
	// Records every 21s, so no window lasts exactly 100s.
	run := runFromPaces([]float64{0.3, 0.3}, 700)

	got := FarthestEfforts(run, []NamedDuration{{"100s", 100 * time.Second}})

	if len(got) != 1 {
		t.Fatalf("got %d efforts, want 1", len(got))
	}
	if got[0].Duration < 100*time.Second {
		t.Errorf("lasted %v, want >= 100s", got[0].Duration)
	}
}

func TestSegmentKind(t *testing.T) {
	for name, want := range map[string]string{"5k": KindDistance, "30min": KindDuration, "1h": KindDuration} {
		if kind, ok := SegmentKind(name); !ok || kind != want {
			t.Errorf("SegmentKind(%s) = %q, %v; want %q", name, kind, ok, want)
		}
	}
	if _, ok := SegmentKind("4k"); ok {
		t.Error("SegmentKind(4k) should not exist")
	}
}
