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
	{"3k", 3000},
	{"2mi", 3218.688},
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

// NamedDuration is an effort measured the other way round: how far you got in a fixed time.
type NamedDuration struct {
	Name     string
	Duration time.Duration
}

// StandardDurations are ranked by distance covered; their names share the segment namespace with StandardDistances.
var StandardDurations = []NamedDuration{
	{"30min", 30 * time.Minute},
	{"1h", time.Hour},
}

func StandardDuration(name string) (NamedDuration, bool) {
	for _, d := range StandardDurations {
		if d.Name == name {
			return d, true
		}
	}
	return NamedDuration{}, false
}

// Segment kinds, as stored: a distance segment is ranked by time, a duration segment by distance.
const (
	KindDistance = "distance"
	KindDuration = "duration"
)

// SegmentKind reports which kind a standard segment name is, and false for an unknown name.
func SegmentKind(name string) (string, bool) {
	if _, ok := StandardDistance(name); ok {
		return KindDistance, true
	}
	if _, ok := StandardDuration(name); ok {
		return KindDuration, true
	}
	return "", false
}

type BestSegment struct {
	Kind     string
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
	idx := withDistance(run)
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
					Kind:     KindDistance,
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

// FarthestEfforts finds, for each target, the stretch of records at least that long covering the most distance.
// Targets longer than the run are omitted.
// Like FastestSegments, the stretch is measured between recorded points, so it can overshoot the target duration by up to one record interval.
func FarthestEfforts(run *Run, targets []NamedDuration) []*BestSegment {
	idx := withDistance(run)
	meters := func(k int) float64 { return run.Records[idx[k]].Distance.Meters }
	offset := func(k int) time.Duration { return run.Records[idx[k]].DurationFromStart }

	var results []*BestSegment
	for _, target := range targets {
		var best *BestSegment
		left := 0
		for right := range idx {
			// Shrink from the left while the window would still last the target.
			for left+1 < right && offset(right)-offset(left+1) >= target.Duration {
				left++
			}
			if offset(right)-offset(left) < target.Duration {
				continue
			}
			covered := meters(right) - meters(left)
			duration := offset(right) - offset(left)
			// On a tie the shorter window is the better effort.
			if best == nil || covered > best.Distance.Meters || (covered == best.Distance.Meters && duration < best.Duration) {
				best = &BestSegment{
					Kind:     KindDuration,
					Distance: &Distance{covered},
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

// withDistance is the indices of records with a distance; records with no fix decode as NaN and would stall a window.
func withDistance(run *Run) []int {
	var idx []int
	for i, rec := range run.Records {
		if rec.Distance != nil && !math.IsNaN(rec.Distance.Meters) {
			idx = append(idx, i)
		}
	}
	return idx
}
