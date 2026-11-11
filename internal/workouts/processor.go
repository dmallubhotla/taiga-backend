package workouts

import (
	"crypto/sha256"
	"fmt"
)

// extractorVersion is bumped whenever what's extracted from a FIT file changes: new fields, or a fix to summaries or segments.
const extractorVersion = 5

// ProcessorVersion identifies the code that built a workout's derived data; workouts stamped with any other value get reprocessed.
// It covers StandardDistances and StandardDurations too, so editing either list reprocesses old workouts without a manual bump.
var ProcessorVersion = processorVersion(extractorVersion, StandardDistances, StandardDurations)

func processorVersion(extractor int, distances []NamedDistance, durations []NamedDuration) string {
	h := sha256.New()
	for _, d := range distances {
		_, _ = fmt.Fprintf(h, "%s=%g;", d.Name, d.Meters)
	}
	for _, d := range durations {
		_, _ = fmt.Fprintf(h, "%s=%gs;", d.Name, d.Duration.Seconds())
	}
	return fmt.Sprintf("%d-%x", extractor, h.Sum(nil)[:4])
}
