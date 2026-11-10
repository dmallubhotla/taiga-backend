package workouts

import (
	"crypto/sha256"
	"fmt"
)

// extractorVersion is bumped whenever what's extracted from a FIT file changes: new fields, or a fix to summaries or segments.
const extractorVersion = 1

// ProcessorVersion identifies the code that built a workout's derived data; workouts stamped with any other value get reprocessed.
// It covers StandardDistances too, so editing that list reprocesses old workouts without a manual bump.
var ProcessorVersion = processorVersion(extractorVersion, StandardDistances)

func processorVersion(extractor int, distances []NamedDistance) string {
	h := sha256.New()
	for _, d := range distances {
		_, _ = fmt.Fprintf(h, "%s=%g;", d.Name, d.Meters)
	}
	return fmt.Sprintf("%d-%x", extractor, h.Sum(nil)[:4])
}
