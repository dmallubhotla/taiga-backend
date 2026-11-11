package workouts

import (
	"testing"
	"time"
)

func TestPlausible(t *testing.T) {
	effort := func(name string, meters, seconds float64) *BestSegment {
		return &BestSegment{Name: name, Distance: &Distance{meters}, Duration: time.Duration(seconds * float64(time.Second))}
	}
	efforts := []*BestSegment{
		effort("1k", 1000, 120),     // 8.3 m/s
		effort("1mi", 1609, 300),    // 5.4 m/s
		effort("30min", 6000, 1800), // 3.3 m/s
	}
	for sport, want := range map[string][]string{
		SportRunning: {"1mi", "30min"},
		SportWalking: {"30min"},
		"cycling":    {"1k", "1mi", "30min"},
	} {
		got := Plausible(sport, efforts)
		if len(got) != len(want) {
			t.Fatalf("%s: kept %d efforts, want %v", sport, len(got), want)
		}
		for i, e := range got {
			if e.Name != want[i] {
				t.Errorf("%s: kept %s, want %s", sport, e.Name, want[i])
			}
		}
	}
}

func TestKnownSport(t *testing.T) {
	for name, want := range map[string]bool{"running": true, "walking": true, "cycling": true, "hiking": true, "Running": false, "jogging": false, "": false} {
		if got := KnownSport(name); got != want {
			t.Errorf("KnownSport(%q) = %v, want %v", name, got, want)
		}
	}
}
