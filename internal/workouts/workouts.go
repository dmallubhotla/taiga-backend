package workouts

import (
	"fmt"
	"time"
)

func metersToMiles(meters float64) float64 {
	return meters / 1609.344
}

type Distance struct {
	Meters float64 `json:"meters"`
}

func (d *Distance) AsMiles() float64 {
	return metersToMiles(d.Meters)
}

func (d *Distance) String() string {
	miles := d.AsMiles()
	return fmt.Sprintf("%.2fmi", miles)
}

type RunSummary struct {
	DistanceMiles  float64   `json:"distance_miles"`
	TimeSeconds    float64   `json:"time_seconds"`
	SpeedMph       float64   `json:"speed_mile_per_hour"`
	PaceMinPerMile float64   `json:"pace_min_per_mile"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
}

// eventually this will get stuff like lat long points
// really we're just 'paraphrasing their struct format
type RecordData struct {
	Distance          *Distance     `json:"distance"`
	Timestamp         time.Time     `json:"timestamp"`
	SpeedMeterPerSec  float64       `json:"-"`
	DurationFromStart time.Duration `json:"duration_from_start"`
}

type Run struct {
	Summary RunSummary    `json:"summary"`
	Records []*RecordData `json:"records"`
}

func (r *Run) String() string {
	return fmt.Sprintf("Run at %v", r.Summary.StartTime)
}
