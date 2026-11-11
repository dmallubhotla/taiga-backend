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
	// Sport and SubSport are the FIT names, e.g. "running" and "indoor_running"; empty when the file doesn't say.
	Sport    string `json:"sport"`
	SubSport string `json:"sub_sport"`
}

// RecordData is one FIT record; fields the device didn't record are NaN.
type RecordData struct {
	Distance          *Distance     `json:"distance"`
	Timestamp         time.Time     `json:"timestamp"`
	SpeedMeterPerSec  float64       `json:"-"`
	AltitudeMeters    float64       `json:"-"`
	HeartRate         float64       `json:"-"`
	Latitude          float64       `json:"-"`
	Longitude         float64       `json:"-"`
	DurationFromStart time.Duration `json:"duration_from_start"`
}

type Run struct {
	Summary RunSummary    `json:"summary"`
	Records []*RecordData `json:"records"`
}

func (r *Run) String() string {
	return fmt.Sprintf("Run at %v", r.Summary.StartTime)
}

// Sports that get best efforts and routes, each kept apart from the other; anything else (rides, generic sessions) is stored and listed but not scored.
const (
	SportRunning = "running"
	SportWalking = "walking"
)

var ScoredSports = []string{SportRunning, SportWalking}

func Scored(sport string) bool {
	return sport == SportRunning || sport == SportWalking
}
