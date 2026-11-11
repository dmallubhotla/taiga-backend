package testsupport

import (
	"encoding/json"
	"testing"
	"time"
)

// Endomondo is a workout as an Endomondo export writes it, built along EncodeFIT's track; tweak its fields to make a bad one.
type Endomondo struct {
	Sport      string
	Source     string
	Start      time.Time
	Duration   time.Duration
	DistanceKm float64
	Points     []EndomondoPoint
}

type EndomondoPoint struct {
	Time       time.Time
	DistanceKm float64
	Lat, Lon   float64
}

// NewEndomondo records splits as sport (e.g. "RUNNING"), tracked by the phone.
func NewEndomondo(start time.Time, splits []Split, sport string) *Endomondo {
	activity := buildActivity(start, splits, 0)
	sesh := activity.Sessions[0]
	e := &Endomondo{
		Sport:      sport,
		Source:     "TRACK_MOBILE",
		Start:      start,
		Duration:   time.Duration(sesh.TotalElapsedTimeScaled() * float64(time.Second)),
		DistanceKm: sesh.TotalDistanceScaled() / 1000,
	}
	for _, rec := range activity.Records {
		e.Points = append(e.Points, EndomondoPoint{Time: rec.Timestamp, DistanceKm: rec.DistanceScaled() / 1000, Lat: rec.PositionLatDegrees(), Lon: rec.PositionLongDegrees()})
	}
	return e
}

// Encode writes Endomondo's shape: a list of single-key objects, each point likewise, with both of its timestamp formats.
func (e *Endomondo) Encode(t *testing.T) []byte {
	t.Helper()
	const summaryTime = "2006-01-02 15:04:05.0"
	type obj = map[string]any
	points := make([][]obj, 0, len(e.Points))
	for _, p := range e.Points {
		points = append(points, []obj{
			{"location": [][]obj{{{"latitude": p.Lat}, {"longitude": p.Lon}}}},
			{"distance_km": p.DistanceKm},
			{"timestamp": p.Time.UTC().Format(time.UnixDate)},
			{"heart_rate_bpm": RecordHeartRate},
		})
	}
	raw, err := json.Marshal([]obj{
		{"sport": e.Sport},
		{"source": e.Source},
		{"start_time": e.Start.UTC().Format(summaryTime)},
		{"end_time": e.Start.Add(e.Duration).UTC().Format(summaryTime)},
		{"duration_s": e.Duration.Seconds()},
		{"distance_km": e.DistanceKm},
		{"points": points},
	})
	if err != nil {
		t.Fatalf("encoding Endomondo JSON: %v", err)
	}
	return raw
}
