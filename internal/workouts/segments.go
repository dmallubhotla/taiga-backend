package workouts

import (
	"fmt"
	"math"
	"time"
)

type BestSegment struct {
	Distance *Distance
	Name     string
	// StartTimestamp time.Time
	Duration time.Duration
	Run      *Run
	Start    int
	End      int
}

func (bs *BestSegment) String() string {
	startDist := bs.Run.Records[bs.Start].DurationFromStart
	endDist := bs.Run.Records[bs.End].DurationFromStart
	return fmt.Sprintf("Best %v in %v, from %v to %v", bs.Distance, bs.Duration, startDist, endDist)
}

func FastestSegments(run *Run, targetDistanceMeters []float64) []*BestSegment {

	records := run.Records

	results := make([]*BestSegment, len(targetDistanceMeters))
	for i, td := range targetDistanceMeters {
		results[i] = &BestSegment{Distance: &Distance{td}, Run: run, Duration: time.Duration(math.MaxInt64)}
	}

	// most efficient to do multiple targets, but need to store an array of lefts for each distance
	lefts := make([]int, len(targetDistanceMeters))

	for right, rightRecord := range records {
		// for each target, we will see if we can advance the left, and if valid see if it helps.
		for i, targetDistance := range targetDistanceMeters {

			segDistance := rightRecord.Distance.Meters - records[lefts[i]].Distance.Meters

			if segDistance > targetDistance {

				for lefts[i] < right && rightRecord.Distance.Meters-records[lefts[i]].Distance.Meters > targetDistance {
					lefts[i]++
				}

				duration := rightRecord.DurationFromStart - records[lefts[i]].DurationFromStart

				if duration < results[i].Duration {
					results[i].Duration = duration
					results[i].Start = lefts[i]
					results[i].End = right
				}
			}
		}
	}

	return results
}
