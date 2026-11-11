package workouts

import (
	"testing"
	"time"
)

func TestProcessorVersionTracksSegments(t *testing.T) {
	version := func(distances []NamedDistance, durations []NamedDuration) string {
		return processorVersion(extractorVersion, distances, durations)
	}
	base := version(StandardDistances, StandardDurations)
	if base != ProcessorVersion {
		t.Errorf("ProcessorVersion = %q, want %q", ProcessorVersion, base)
	}
	if version(StandardDistances, StandardDurations) != base {
		t.Error("processorVersion isn't deterministic")
	}
	if processorVersion(extractorVersion+1, StandardDistances, StandardDurations) == base {
		t.Error("bumping the extractor version didn't change ProcessorVersion")
	}
	added := append(append([]NamedDistance{}, StandardDistances...), NamedDistance{"15k", 15000})
	if version(added, StandardDurations) == base {
		t.Error("adding a distance didn't change ProcessorVersion")
	}
	renamed := append([]NamedDistance{}, StandardDistances...)
	renamed[0] = NamedDistance{"1km", renamed[0].Meters}
	if version(renamed, StandardDurations) == base {
		t.Error("renaming a distance didn't change ProcessorVersion")
	}
	longer := append(append([]NamedDuration{}, StandardDurations...), NamedDuration{"2h", 2 * time.Hour})
	if version(StandardDistances, longer) == base {
		t.Error("adding a duration didn't change ProcessorVersion")
	}
}
