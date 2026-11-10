package workouts

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
)

var (
	// ErrUnreadableFile means the bytes aren't a FIT file we can decode.
	ErrUnreadableFile = errors.New("not a readable FIT file")
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
	}
}

func summariseRecord(rec *mesgdef.Record) *RecordData {

	distance := &Distance{
		Meters: rec.DistanceScaled(),
	}

	return &RecordData{
		Distance:         distance,
		Timestamp:        rec.Timestamp,
		SpeedMeterPerSec: rec.SpeedScaled(),
	}
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
