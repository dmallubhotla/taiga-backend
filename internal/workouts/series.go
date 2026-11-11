package workouts

import (
	"math"
	"strconv"
)

// MaxSeriesPoints caps a stored series; plenty for a graph or a map track, and it keeps a marathon's 1s records small.
const MaxSeriesPoints = 2000

// Values is a series column; NaN (not recorded) encodes as null.
type Values []float64

func (v Values) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, len(v)*8+2)
	buf = append(buf, '[')
	for i, x := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		if math.IsNaN(x) || math.IsInf(x, 0) {
			buf = append(buf, "null"...)
			continue
		}
		buf = strconv.AppendFloat(buf, x, 'f', -1, 64)
	}
	return append(buf, ']'), nil
}

// Series is a workout's records as parallel columns, the shape graphing libraries take.
// A column the device never recorded (heart rate without a strap, position on a treadmill) is omitted.
type Series struct {
	TimeSeconds    Values `json:"time_s"`
	DistanceMeters Values `json:"distance_m"`
	SpeedMPS       Values `json:"speed_mps,omitempty"`
	AltitudeMeters Values `json:"altitude_m,omitempty"`
	HeartRate      Values `json:"heart_rate,omitempty"`
	Latitude       Values `json:"lat,omitempty"`
	Longitude      Values `json:"lon,omitempty"`
}

// BuildSeries samples the run's records evenly down to at most maxPoints, always keeping the last.
func BuildSeries(run *Run, maxPoints int) *Series {
	n := len(run.Records)
	stride := 1
	if n > maxPoints {
		stride = (n + maxPoints - 2) / (maxPoints - 1)
	}
	var picked []*RecordData
	for i := 0; i < n; i += stride {
		picked = append(picked, run.Records[i])
	}
	if n > 0 && (n-1)%stride != 0 {
		picked = append(picked, run.Records[n-1])
	}

	column := func(get func(*RecordData) float64, decimals int) Values {
		out := make(Values, len(picked))
		any := false
		for i, r := range picked {
			out[i] = round(get(r), decimals)
			any = any || !math.IsNaN(out[i])
		}
		if !any {
			return nil
		}
		return out
	}
	s := &Series{
		TimeSeconds: column(func(r *RecordData) float64 { return r.DurationFromStart.Seconds() }, 1),
		DistanceMeters: column(func(r *RecordData) float64 {
			if r.Distance == nil {
				return math.NaN()
			}
			return r.Distance.Meters
		}, 1),
		SpeedMPS:       column(func(r *RecordData) float64 { return r.SpeedMeterPerSec }, 3),
		AltitudeMeters: column(func(r *RecordData) float64 { return r.AltitudeMeters }, 1),
		HeartRate:      column(func(r *RecordData) float64 { return r.HeartRate }, 0),
		// Six decimals of a degree is about 10cm.
		Latitude:  column(func(r *RecordData) float64 { return r.Latitude }, 6),
		Longitude: column(func(r *RecordData) float64 { return r.Longitude }, 6),
	}
	// Time and distance are the x axes, so keep them present even when empty.
	if s.TimeSeconds == nil {
		s.TimeSeconds = Values{}
	}
	if s.DistanceMeters == nil {
		s.DistanceMeters = make(Values, len(picked))
		for i := range s.DistanceMeters {
			s.DistanceMeters[i] = math.NaN()
		}
	}
	return s
}

func round(x float64, decimals int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return math.NaN()
	}
	p := math.Pow(10, float64(decimals))
	return math.Round(x*p) / p
}
