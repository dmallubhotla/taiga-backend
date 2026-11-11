package workouts

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/muktihari/fit/profile/typedef"
)

// endomondoSports maps Endomondo's sport names onto FIT sport and sub-sport names; anything else is generic.
var endomondoSports = map[string][2]string{
	"RUNNING":           {typedef.SportRunning.String(), ""},
	"TREADMILL_RUNNING": {typedef.SportRunning.String(), typedef.SubSportTreadmill.String()},
	"WALKING":           {typedef.SportWalking.String(), ""},
	"TREADMILL_WALKING": {typedef.SportWalking.String(), typedef.SubSportIndoorWalking.String()},
	"HIKING":            {typedef.SportHiking.String(), ""},
	"CYCLING_SPORT":     {typedef.SportCycling.String(), typedef.SubSportRoad.String()},
	"CYCLING_TRANSPORT": {typedef.SportCycling.String(), ""},
	"MOUNTAIN_BIKING":   {typedef.SportCycling.String(), typedef.SubSportMountain.String()},
	"SPINNING":          {typedef.SportCycling.String(), typedef.SubSportIndoorCycling.String()},
	"STAIR_CLIMBING":    {typedef.SportFitnessEquipment.String(), typedef.SubSportStairClimbing.String()},
	"WEIGHT_TRAINING":   {typedef.SportTraining.String(), typedef.SubSportStrengthTraining.String()},
	"AEROBICS":          {typedef.SportTraining.String(), typedef.SubSportCardioTraining.String()},
	"CROSS_TRAINING":    {typedef.SportTraining.String(), typedef.SubSportCardioTraining.String()},
	"ROWING":            {typedef.SportRowing.String(), ""},
	"ROWING_INDOOR":     {typedef.SportRowing.String(), typedef.SubSportIndoorRowing.String()},
	"SWIMMING":          {typedef.SportSwimming.String(), ""},
}

// Endomondo writes both forms, sometimes within one export: "2020-07-23 22:20:14.0" and "Thu Jul 23 22:20:14 UTC 2020".
var endomondoTimeLayouts = []string{"2006-01-02 15:04:05", time.UnixDate}

// Why an Endomondo workout doesn't count (RunSummary.ExcludedReason).
const (
	ExcludedHandEntered    = "hand-entered"
	ExcludedTimeBackwards  = "time runs backwards"
	ExcludedPointsDisagree = "points disagree with totals"
	ExcludedNoDistance     = "no distance recorded"
)

// pointsAgreeWithin is how far the points' last distance may stray from the stated total.
const pointsAgreeWithin = 0.1

type endomondoWorkout struct {
	Sport      string                         `json:"sport"`
	Source     string                         `json:"source"`
	StartTime  string                         `json:"start_time"`
	EndTime    string                         `json:"end_time"`
	DurationS  float64                        `json:"duration_s"`
	DistanceKm float64                        `json:"distance_km"`
	Points     [][]map[string]json.RawMessage `json:"points"`
}

type endomondoPoint struct {
	Location   [][]map[string]float64 `json:"location"`
	Timestamp  string                 `json:"timestamp"`
	DistanceKm *float64               `json:"distance_km"`
	Altitude   *float64               `json:"altitude"`
	SpeedKmh   *float64               `json:"speed_kmh"`
	HeartRate  *float64               `json:"heart_rate_bpm"`
}

// ReadEndomondoFile reads one workout from an Endomondo export's JSON.
// The stated totals are the summary. A workout whose data can't be trusted (hand-entered, a run with no distance, time running backwards, points that disagree with the totals) keeps those totals but no points, and an ExcludedReason so it never counts.
func ReadEndomondoFile(r io.Reader) (*Run, error) {
	var entries []map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&entries); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadableFile, err)
	}
	var w endomondoWorkout
	if err := unmarshalMerged(entries, &w); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadableFile, err)
	}
	start, err := parseEndomondoTime(w.StartTime)
	if err != nil {
		return nil, fmt.Errorf("%w: start_time: %v", ErrUnreadableFile, err)
	}

	duration := time.Duration(w.DurationS * float64(time.Second))
	// A hand-entered workout's end_time is seconds after its start whatever its duration.
	end := start.Add(duration)
	if parsed, err := parseEndomondoTime(w.EndTime); err == nil && parsed.After(end) {
		end = parsed
	}
	distance := &Distance{Meters: w.DistanceKm * 1000}
	hours := duration.Hours()
	sport := endomondoSports[w.Sport]
	if sport[0] == "" {
		sport[0] = typedef.SportGeneric.String()
	}

	run := &Run{Summary: RunSummary{
		DistanceMiles:  distance.AsMiles(),
		TimeSeconds:    duration.Seconds(),
		SpeedMph:       distance.AsMiles() / hours,
		PaceMinPerMile: hours * 60 / distance.AsMiles(),
		StartTime:      start,
		EndTime:        end,
		Sport:          sport[0],
		SubSport:       sport[1],
	}}

	backwards := false
	for _, raw := range w.Points {
		var p endomondoPoint
		if err := unmarshalMerged(raw, &p); err != nil {
			return nil, fmt.Errorf("%w: point: %v", ErrUnreadableFile, err)
		}
		ts, err := parseEndomondoTime(p.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("%w: point timestamp: %v", ErrUnreadableFile, err)
		}
		if n := len(run.Records); n > 0 && ts.Before(run.Records[n-1].Timestamp) {
			backwards = true
		}
		rec := &RecordData{
			Distance:          &Distance{Meters: orNaN(p.DistanceKm) * 1000},
			Timestamp:         ts,
			SpeedMeterPerSec:  orNaN(p.SpeedKmh) / 3.6,
			AltitudeMeters:    orNaN(p.Altitude),
			HeartRate:         orNaN(p.HeartRate),
			Latitude:          math.NaN(),
			Longitude:         math.NaN(),
			DurationFromStart: ts.Sub(start),
		}
		if len(p.Location) > 0 {
			var loc struct{ Latitude, Longitude *float64 }
			if err := unmarshalMerged(p.Location[0], &loc); err == nil && loc.Latitude != nil && loc.Longitude != nil {
				rec.Latitude, rec.Longitude = *loc.Latitude, *loc.Longitude
			}
		}
		run.Records = append(run.Records, rec)
	}
	switch {
	case strings.HasPrefix(w.Source, "INPUT_MANUAL"):
		run.Summary.ExcludedReason = ExcludedHandEntered
	case Scored(run.Summary.Sport) && distance.Meters == 0:
		run.Summary.ExcludedReason = ExcludedNoDistance
	case backwards:
		run.Summary.ExcludedReason = ExcludedTimeBackwards
	case !pointsAgree(run.Records, distance.Meters, start):
		run.Summary.ExcludedReason = ExcludedPointsDisagree
	}
	if run.Summary.ExcludedReason != "" {
		run.Records = nil
	}
	return run, nil
}

// pointsAgree reports whether the points fit the stated totals: none before the start, and covering its distance.
// Points may run past end_time, which Endomondo set to the start plus moving time, so pauses push them later.
func pointsAgree(records []*RecordData, meters float64, start time.Time) bool {
	if len(records) > 0 && records[0].Timestamp.Before(start.Add(-time.Minute)) {
		return false
	}
	covered := 0.0
	for _, rec := range records {
		if !math.IsNaN(rec.Distance.Meters) {
			covered = max(covered, rec.Distance.Meters)
		}
	}
	return math.Abs(covered-meters) <= pointsAgreeWithin*meters
}

// unmarshalMerged decodes Endomondo's list of single-key objects as if it were one object.
func unmarshalMerged[T any](entries []map[string]T, v any) error {
	merged := map[string]T{}
	for _, e := range entries {
		for k, val := range e {
			merged[k] = val
		}
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func parseEndomondoTime(s string) (time.Time, error) {
	var err error
	for _, layout := range endomondoTimeLayouts {
		var t time.Time
		if t, err = time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}

func orNaN(v *float64) float64 {
	if v == nil {
		return math.NaN()
	}
	return *v
}
