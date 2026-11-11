package testsupport

import (
	"encoding/xml"
	"testing"
	"time"
)

// TCX is a session as Google Fit's Takeout writes it, built along EncodeFIT's track: one activity per entry of Sports, splitting the points evenly, each measuring its distance from where the last one ended.
type TCX struct {
	Author string
	Sports []string
	Points []TCXPoint
}

type TCXPoint struct {
	Time     time.Time
	Meters   float64
	Lat, Lon float64
	// NoPosition drops the point's fix, as Google Fit does between sparse fixes.
	NoPosition bool
}

// NewTCX records splits as a Google Fit session of the given TCX sports (e.g. "Running").
func NewTCX(start time.Time, splits []Split, sports ...string) *TCX {
	x := &TCX{Author: "Google Fit", Sports: sports}
	for _, rec := range buildActivity(start, splits, 0).Records {
		x.Points = append(x.Points, TCXPoint{Time: rec.Timestamp, Meters: rec.DistanceScaled(), Lat: rec.PositionLatDegrees(), Lon: rec.PositionLongDegrees()})
	}
	return x
}

type tcxPosition struct {
	Lat float64 `xml:"LatitudeDegrees"`
	Lon float64 `xml:"LongitudeDegrees"`
}

type tcxTrackpoint struct {
	Distance float64      `xml:"DistanceMeters"`
	Time     string       `xml:"Time"`
	Position *tcxPosition `xml:"Position,omitempty"`
}

type tcxLap struct {
	StartTime string          `xml:"StartTime,attr"`
	Points    []tcxTrackpoint `xml:"Track>Trackpoint"`
	Distance  float64         `xml:"DistanceMeters"`
	Seconds   float64         `xml:"TotalTimeSeconds"`
}

type tcxActivity struct {
	Sport string `xml:"Sport,attr"`
	ID    string `xml:"Id"`
	Lap   tcxLap `xml:"Lap"`
}

type tcxDoc struct {
	XMLName    xml.Name      `xml:"TrainingCenterDatabase"`
	Activities []tcxActivity `xml:"Activities>Activity"`
	Author     string        `xml:"Author>Name"`
}

func (x *TCX) Encode(t *testing.T) []byte {
	t.Helper()
	doc := tcxDoc{Author: x.Author}
	per := (len(x.Points) + len(x.Sports) - 1) / len(x.Sports)
	base := x.Points[0]
	for i, sport := range x.Sports {
		chunk := x.Points[min(i*per, len(x.Points)):min((i+1)*per, len(x.Points))]
		if len(chunk) == 0 {
			continue
		}
		last := chunk[len(chunk)-1]
		lap := tcxLap{
			StartTime: base.Time.Format(time.RFC3339Nano),
			Distance:  last.Meters - base.Meters,
			Seconds:   last.Time.Sub(base.Time).Seconds(),
		}
		for _, p := range chunk {
			tp := tcxTrackpoint{Distance: p.Meters - base.Meters, Time: p.Time.Format(time.RFC3339Nano)}
			if !p.NoPosition {
				tp.Position = &tcxPosition{p.Lat, p.Lon}
			}
			lap.Points = append(lap.Points, tp)
		}
		doc.Activities = append(doc.Activities, tcxActivity{Sport: sport, ID: lap.StartTime, Lap: lap})
		base = last
	}
	b, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("encoding TCX: %v", err)
	}
	return append([]byte(xml.Header), b...)
}
