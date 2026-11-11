package workouts

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
)

var (
	// ErrUnreadableFile means the bytes aren't a FIT, Endomondo JSON or TCX file we can decode.
	ErrUnreadableFile = errors.New("not a readable activity file (FIT, TCX or Endomondo JSON)")
	// ErrUnsupportedActivity means the file decodes but isn't a single-session activity.
	ErrUnsupportedActivity = errors.New("unsupported activity")
)

func summarise(sesh *mesgdef.Session) *RunSummary {

	totalDistance := &Distance{sesh.TotalDistanceScaled()}
	distanceMiles := totalDistance.AsMiles()
	timeSeconds := sesh.TotalElapsedTimeScaled()
	// Prefer the recorded elapsed time; some files close the session with Timestamp == StartTime.
	hours := timeSeconds / 3600
	if math.IsNaN(hours) || hours <= 0 {
		hours = sesh.Timestamp.Sub(sesh.StartTime).Hours()
	}

	speedMph := distanceMiles / hours
	paceMinPerMile := hours * 60 / distanceMiles

	return &RunSummary{
		DistanceMiles:  distanceMiles,
		TimeSeconds:    timeSeconds,
		SpeedMph:       speedMph,
		PaceMinPerMile: paceMinPerMile,
		EndTime:        sesh.Timestamp,
		StartTime:      sesh.StartTime,
		Sport:          sportName(sesh.Sport),
		SubSport:       subSportName(sesh.SubSport),
	}
}

// sportName is the FIT sport as stored, e.g. "running"; empty when the device didn't say.
func sportName(s typedef.Sport) string {
	if s == typedef.SportInvalid {
		return ""
	}
	return s.String()
}

// subSportName is the FIT sub-sport, e.g. "indoor_running"; empty when there's none beyond the sport.
func subSportName(s typedef.SubSport) string {
	if s == typedef.SubSportInvalid || s == typedef.SubSportGeneric {
		return ""
	}
	return s.String()
}

// fitSource names the device or app that wrote the file: its product name when it gives one (COROS does), else the manufacturer and, for Garmin, the product.
func fitSource(id *mesgdef.FileId) string {
	if name := strings.TrimSpace(id.ProductName); name != "" {
		return name
	}
	if id.Manufacturer == typedef.ManufacturerInvalid || strings.HasPrefix(id.Manufacturer.String(), "ManufacturerInvalid") {
		return ""
	}
	source := strings.ReplaceAll(id.Manufacturer.String(), "_", " ")
	source = strings.ToUpper(source[:1]) + source[1:]
	if _, product := id.GetProduct(); product != nil {
		if p, ok := product.(typedef.GarminProduct); ok && !strings.HasPrefix(p.String(), "GarminProductInvalid") {
			source += " " + p.String()
		}
	}
	return source
}

func summariseRecord(rec *mesgdef.Record) *RecordData {
	return &RecordData{
		Distance:         &Distance{Meters: rec.DistanceScaled()},
		Timestamp:        rec.Timestamp,
		SpeedMeterPerSec: firstValid(rec.EnhancedSpeedScaled(), rec.SpeedScaled()),
		AltitudeMeters:   firstValid(rec.EnhancedAltitudeScaled(), rec.AltitudeScaled()),
		HeartRate:        validUint8(rec.HeartRate),
		Latitude:         rec.PositionLatDegrees(),
		Longitude:        rec.PositionLongDegrees(),
	}
}

// firstValid prefers the enhanced (wider) field, falling back to the legacy one older devices write.
func firstValid(preferred, fallback float64) float64 {
	if !math.IsNaN(preferred) {
		return preferred
	}
	return fallback
}

func validUint8(v uint8) float64 {
	if v == basetype.Uint8Invalid {
		return math.NaN()
	}
	return float64(v)
}

func ReadActivity(activity *filedef.Activity) (*Run, error) {

	run := Run{}

	switch n := len(activity.Sessions); {
	case n == 0:
		return nil, fmt.Errorf("%w: the file has no session", ErrUnsupportedActivity)
	case n > 1:
		return nil, fmt.Errorf("%w: the file has %d sessions and multi-sport activities aren't supported yet", ErrUnsupportedActivity, n)
	}
	onlySession := activity.Sessions[0]
	summary := summarise(onlySession)
	summary.Source = fitSource(&activity.FileId)
	run.Summary = *summary

	for _, rec := range activity.Records {
		// rec := activity.Records[i]
		rec_summary := summariseRecord(rec)
		rec_summary.DurationFromStart = rec_summary.Timestamp.Sub(run.Summary.StartTime)

		run.Records = append(run.Records, rec_summary)

		// log.Printf("%v", rec.SpeedScaled())
		// log.Printf("%v", rec.Speed)
		// log.Printf("record: %+v", summary)
	}

	return &run, nil
}
func ReadFitFile(r io.Reader) (*Run, error) {

	dec := decoder.New(r)
	fit, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadableFile, err)
	}

	activity := filedef.NewActivity(fit.Messages...)
	run, err := ReadActivity(activity)
	if err != nil {
		return nil, err
	}
	return run, nil
}
