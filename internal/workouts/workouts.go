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
	// ExcludedReason keeps a run or walk out of best efforts, routes and stats when its data can't be trusted, e.g. "hand-entered"; empty when it counts.
	ExcludedReason string `json:"excluded_reason"`
	// Source is what recorded the workout, e.g. "COROS PACE 3" or "Endomondo"; empty when the file doesn't say.
	Source string `json:"source"`
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

// maxSpeedMps caps each scored sport: an effort faster than this is a mislabeled activity or a GPS jump, so it isn't stored.
// Running's is about the 1k world record (2:11.96), walking's the race-walking mile record (5:31).
var maxSpeedMps = map[string]float64{SportRunning: 7.6, SportWalking: 4.9}

// Plausible drops efforts faster than the sport's cap; sports without one keep all of theirs.
func Plausible(sport string, efforts []*BestSegment) []*BestSegment {
	limit, ok := maxSpeedMps[sport]
	if !ok {
		return efforts
	}
	kept := make([]*BestSegment, 0, len(efforts))
	for _, e := range efforts {
		if e.Distance.Meters <= limit*e.Duration.Seconds() {
			kept = append(kept, e)
		}
	}
	return kept
}

// Indoor sub-sports measure distance with a foot pod or the belt, not GPS, so they never count towards best efforts or routes.
var indoorSubSports = map[string]bool{"treadmill": true, "indoor_running": true, "indoor_walking": true}

// Scored reports whether this session gets best efforts and a route: a run or walk, outdoors, with data that can be trusted.
func (s *RunSummary) Scored() bool {
	return Scored(s.Sport) && !indoorSubSports[s.SubSport] && s.ExcludedReason == ""
}
