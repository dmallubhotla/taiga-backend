package workouts

import (
	"fmt"
	"math"
	"time"
)

type NamedDistance struct {
	Name   string
	Meters float64
}

var StandardDistances = []NamedDistance{
	{"1k", 1000},
	{"1mi", 1609.344},
	{"5k", 5000},
	{"5mi", 8046.72},
	{"10k", 10000},
	{"half", 21097.5},
	{"marathon", 42195},
}

func StandardDistance(name string) (NamedDistance, bool) {
	for _, d := range StandardDistances {
		if d.Name == name {
			return d, true
		}
	}
	return NamedDistance{}, false
}

type BestSegment struct {
	Distance *Distance
	Name     string
	Duration time.Duration
	Run      *Run
	// Indices into Run.Records.
	Start int
	End   int
}

func (bs *BestSegment) StartOffset() time.Duration {
	return bs.Run.Records[bs.Start].DurationFromStart
}

func (bs *BestSegment) EndOffset() time.Duration {
	return bs.Run.Records[bs.End].DurationFromStart
}

func (bs *BestSegment) String() string {
	return fmt.Sprintf("Best %v in %v, from %v to %v", bs.Distance, bs.Duration, bs.StartOffset(), bs.EndOffset())
}

// FastestSegments finds, for each target, the quickest stretch of records covering at least that distance.
// Targets the run never covers are omitted.
// The stretch is measured between recorded points, so it can overshoot the target by up to one record interval.
func FastestSegments(run *Run, targets []NamedDistance) []*BestSegment {
	// Records with no distance fix decode as NaN and would stall the window.
	var idx []int
	for i, rec := range run.Records {
		if rec.Distance != nil && !math.IsNaN(rec.Distance.Meters) {
			idx = append(idx, i)
		}
	}
	meters := func(k int) float64 { return run.Records[idx[k]].Distance.Meters }
	offset := func(k int) time.Duration { return run.Records[idx[k]].DurationFromStart }

	var results []*BestSegment
	for _, target := range targets {
		var best *BestSegment
		left := 0
		for right := range idx {
			// Shrink from the left while the window would still cover the target.
			for left+1 < right && meters(right)-meters(left+1) >= target.Meters {
				left++
			}
			if meters(right)-meters(left) < target.Meters {
				continue
			}
			duration := offset(right) - offset(left)
			if best == nil || duration < best.Duration {
				best = &BestSegment{
					Distance: &Distance{target.Meters},
					Name:     target.Name,
					Duration: duration,
					Run:      run,
					Start:    idx[left],
					End:      idx[right],
				}
			}
		}
		if best != nil {
			results = append(results, best)
		}
	}
	return results
}
