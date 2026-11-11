package workouts

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/muktihari/fit/profile/typedef"
)

// tcxSports maps TCX sport names (Google Fit writes its own, e.g. "Running.treadmill") onto FIT sport and sub-sport names; anything else is generic.
var tcxSports = map[string][2]string{
	"Running":           {typedef.SportRunning.String(), ""},
	"Running.treadmill": {typedef.SportRunning.String(), typedef.SubSportTreadmill.String()},
	"Walking":           {typedef.SportWalking.String(), ""},
	"Biking":            {typedef.SportCycling.String(), ""},
	"Biking.stationary": {typedef.SportCycling.String(), typedef.SubSportIndoorCycling.String()},
	"Biking.spinning":   {typedef.SportCycling.String(), typedef.SubSportSpin.String()},
	"Strength_training": {typedef.SportTraining.String(), typedef.SubSportStrengthTraining.String()},
	"Weightlifting":     {typedef.SportTraining.String(), typedef.SubSportStrengthTraining.String()},
	"Calisthenics":      {typedef.SportTraining.String(), typedef.SubSportCardioTraining.String()},
	"Aerobics":          {typedef.SportTraining.String(), typedef.SubSportCardioTraining.String()},
	"Stair_climbing":    {typedef.SportFitnessEquipment.String(), typedef.SubSportStairClimbing.String()},
	"Elliptical":        {typedef.SportFitnessEquipment.String(), typedef.SubSportElliptical.String()},
	"Tennis":            {typedef.SportTennis.String(), ""},
	"Meditation":        {typedef.SportMeditation.String(), ""},
}

const (
	ExcludedMixedSports = "mixed sports"
	ExcludedSparseGPS   = "too few GPS points"
)

// tcxMinPointsPerMile separates a tracked run from an auto-detected one: on a Google Fit export tracked runs had 200+ fixes a mile, auto-detected ones under 20.
const tcxMinPointsPerMile = 50

type tcxFile struct {
	XMLName    xml.Name      `xml:"TrainingCenterDatabase"`
	Activities []tcxActivity `xml:"Activities>Activity"`
	Author     string        `xml:"Author>Name"`
}

type tcxActivity struct {
	Sport   string   `xml:"Sport,attr"`
	Creator string   `xml:"Creator>Name"`
	Laps    []tcxLap `xml:"Lap"`
}

type tcxLap struct {
	StartTime     time.Time       `xml:"StartTime,attr"`
	DistanceMeter float64         `xml:"DistanceMeters"`
	TotalSeconds  float64         `xml:"TotalTimeSeconds"`
	Points        []tcxTrackpoint `xml:"Track>Trackpoint"`
}

type tcxTrackpoint struct {
	Time      time.Time `xml:"Time"`
	Distance  *float64  `xml:"DistanceMeters"`
	Latitude  *float64  `xml:"Position>LatitudeDegrees"`
	Longitude *float64  `xml:"Position>LongitudeDegrees"`
	Altitude  *float64  `xml:"AltitudeMeters"`
	HeartRate *float64  `xml:"HeartRateBpm>Value"`
}

// ReadTCXFile reads one session from a TCX file, as Google Fit's Takeout writes them: one file per session, its activities the stretches Google Fit detected.
// Each activity and lap restarts its distance from zero, so the laps are laid end to end.
// A scored session that mixes sports, has time running backwards, or has too few GPS points for its distance was auto-detected rather than tracked: it keeps its totals but no points, and an ExcludedReason.
func ReadTCXFile(r io.Reader) (*Run, error) {
	var f tcxFile
	if err := xml.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadableFile, err)
	}
	var laps []tcxLap
	sports := map[string]bool{}
	source := strings.TrimSpace(f.Author)
	for _, a := range f.Activities {
		sports[a.Sport] = true
		laps = append(laps, a.Laps...)
		if source == "" {
			source = strings.TrimSpace(a.Creator)
		}
	}
	if len(laps) == 0 {
		return nil, fmt.Errorf("%w: the file has no lap", ErrUnsupportedActivity)
	}

	start, end := laps[0].StartTime, laps[0].StartTime
	var offset float64
	var records []*RecordData
	gps := 0
	backwards := false
	for _, lap := range laps {
		if lap.StartTime.Before(start) {
			start = lap.StartTime
		}
		if lapEnd := lap.StartTime.Add(time.Duration(lap.TotalSeconds * float64(time.Second))); lapEnd.After(end) {
			end = lapEnd
		}
		for _, p := range lap.Points {
			if n := len(records); n > 0 && p.Time.Before(records[n-1].Timestamp) {
				backwards = true
			}
			if p.Time.After(end) {
				end = p.Time
			}
			rec := &RecordData{
				Distance:         &Distance{Meters: offset + orNaN(p.Distance)},
				Timestamp:        p.Time,
				SpeedMeterPerSec: math.NaN(),
				AltitudeMeters:   orNaN(p.Altitude),
				HeartRate:        orNaN(p.HeartRate),
				Latitude:         math.NaN(),
				Longitude:        math.NaN(),
			}
			if p.Latitude != nil && p.Longitude != nil {
				rec.Latitude, rec.Longitude = *p.Latitude, *p.Longitude
				gps++
			}
			records = append(records, rec)
		}
		offset += lap.DistanceMeter
	}
	for _, rec := range records {
		rec.DurationFromStart = rec.Timestamp.Sub(start)
	}

	sport := tcxSports[f.Activities[0].Sport]
	if sport[0] == "" {
		sport[0] = typedef.SportGeneric.String()
	}
	distance := &Distance{Meters: offset}
	duration := end.Sub(start)
	hours := duration.Hours()
	run := &Run{
		Summary: RunSummary{
			DistanceMiles:  distance.AsMiles(),
			TimeSeconds:    duration.Seconds(),
			SpeedMph:       distance.AsMiles() / hours,
			PaceMinPerMile: hours * 60 / distance.AsMiles(),
			StartTime:      start,
			EndTime:        end,
			Sport:          sport[0],
			SubSport:       sport[1],
			Source:         source,
		},
		Records: records,
	}
	if Scored(run.Summary.Sport) && !indoorSubSports[run.Summary.SubSport] {
		switch {
		case len(sports) > 1:
			run.Summary.ExcludedReason = ExcludedMixedSports
		case distance.Meters == 0:
			run.Summary.ExcludedReason = ExcludedNoDistance
		case backwards:
			run.Summary.ExcludedReason = ExcludedTimeBackwards
		case float64(gps) < tcxMinPointsPerMile*distance.AsMiles():
			run.Summary.ExcludedReason = ExcludedSparseGPS
		}
	}
	if run.Summary.ExcludedReason != "" {
		run.Records = nil
	}
	return run, nil
}
