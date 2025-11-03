package workouts

import (
	"fmt"
	"io"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
)

func summarise(sesh *mesgdef.Session) *RunSummary {

	totalDistance := &Distance{sesh.TotalDistanceScaled()}
	distanceMiles := totalDistance.AsMiles()
	timeSeconds := sesh.TotalElapsedTimeScaled()
	duration := sesh.Timestamp.Sub(sesh.StartTime)

	speedMph := distanceMiles / duration.Hours()
	paceMinPerMile := duration.Minutes() / distanceMiles

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

	sessions := activity.Sessions
	if len(sessions) != 1 {
		// log.Printf("Issue, got %d sessions, more than one.", len(activity.Sessions))
		return nil, fmt.Errorf("Got more than one session in activity")
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
		return nil, fmt.Errorf("Received error decoding file: %w", err)
	}

	activity := filedef.NewActivity(fit.Messages...)
	run, err := ReadActivity(activity)
	if err != nil {
		return nil, fmt.Errorf("Received error while reading activity: %w", err)
	}
	return run, nil
}
