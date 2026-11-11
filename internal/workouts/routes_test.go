package workouts

import (
	"math"
	"testing"
	"time"
)

// xy is a point in meters east and north of 45°N, 93°W.
type xy struct{ x, y float64 }

var (
	// A 1km square run anticlockwise from the south-west corner.
	square = []xy{{0, 0}, {1000, 0}, {1000, 1000}, {0, 1000}, {0, 0}}
	// Out 1.5km east and back.
	outAndBack = []xy{{0, 0}, {1500, 0}, {0, 0}}
)

func reversed(path []xy) []xy {
	out := make([]xy, len(path))
	for i, p := range path {
		out[len(path)-1-i] = p
	}
	return out
}

func shifted(path []xy, dx, dy float64) []xy {
	out := make([]xy, len(path))
	for i, p := range path {
		out[i] = xy{p.x + dx, p.y + dy}
	}
	return out
}

// runAlong records a fix every 10m along the path, with the odometer reading stretch times the distance travelled (GPS rarely agrees with itself on length).
func runAlong(path []xy, stretch float64) *Run {
	const lat0, lon0 = 45.0, -93.0
	mPerDegLat := earthRadiusM * math.Pi / 180
	mPerDegLon := mPerDegLat * math.Cos(lat0*math.Pi/180)
	run := &Run{}
	var travelled float64
	add := func(p xy) {
		run.Records = append(run.Records, &RecordData{
			Distance:          &Distance{travelled * stretch},
			Latitude:          lat0 + p.y/mPerDegLat,
			Longitude:         lon0 + p.x/mPerDegLon,
			DurationFromStart: time.Duration(travelled*0.3) * time.Second,
		})
	}
	add(path[0])
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		leg := math.Hypot(b.x-a.x, b.y-a.y)
		for s := 10.0; s <= leg+1e-9; s += 10 {
			travelled += 10
			add(xy{a.x + (b.x-a.x)*s/leg, a.y + (b.y-a.y)*s/leg})
		}
	}
	return run
}

func TestBuildTrackResamplesByDistance(t *testing.T) {
	track := BuildTrack(runAlong(square, 1))
	if track == nil {
		t.Fatal("no track")
		return
	}
	if math.Abs(track.Meters-4000) > 1e-6 {
		t.Errorf("Meters = %v, want 4000", track.Meters)
	}
	if want := 4000/TrackSpacingMeters + 1; len(track.Points) != want {
		t.Errorf("%d points, want %d", len(track.Points), want)
	}
	if track.Points[0] != track.Points[len(track.Points)-1] {
		t.Errorf("a loop should end where it started: %v vs %v", track.Points[0], track.Points[len(track.Points)-1])
	}
}

func TestBuildTrackNeedsGPS(t *testing.T) {
	run := runFromPaces([]float64{0.3, 0.3}, 1000)
	for _, r := range run.Records {
		r.Latitude, r.Longitude = math.NaN(), math.NaN()
	}
	if track := BuildTrack(run); track != nil {
		t.Errorf("treadmill run got a track: %+v", track)
	}
	if track := BuildTrack(runAlong([]xy{{0, 0}, {100, 0}}, 1)); track != nil {
		t.Error("a 100m track should be too short to match")
	}
}

func TestSameRoute(t *testing.T) {
	base := BuildTrack(runAlong(square, 1))
	tests := []struct {
		name string
		run  *Run
		want bool
	}{
		{"the same loop", runAlong(square, 1), true},
		{"GPS drifting 30m and reading 5% long", runAlong(shifted(square, 30, 0), 1.05), true},
		{"the loop the other way round", runAlong(reversed(square), 1), false},
		{"a parallel loop a block over", runAlong(shifted(square, 120, 0), 1), false},
		{"the loop with a long out-and-back added", runAlong([]xy{{0, 0}, {1000, 0}, {1000, 500}, {1500, 500}, {1000, 500}, {1000, 1000}, {0, 1000}, {0, 0}}, 1), false},
		{"an out-and-back", runAlong(outAndBack, 1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, got := SameRoute(base, BuildTrack(tt.run))
			if got != tt.want {
				t.Errorf("SameRoute = %v (Fréchet %.1fm), want %v", got, d, tt.want)
			}
		})
	}
}

func TestSameRouteOutAndBackEitherWay(t *testing.T) {
	// Run backwards, an out-and-back retraces the same path, so it's the same route.
	_, ok := SameRoute(BuildTrack(runAlong(outAndBack, 1)), BuildTrack(runAlong(reversed(outAndBack), 1)))
	if !ok {
		t.Error("an out-and-back should match itself reversed")
	}
}

func TestGroupRoutes(t *testing.T) {
	tracks := []*Track{
		BuildTrack(runAlong(square, 1)),
		BuildTrack(runAlong(reversed(square), 1)),
		nil, // treadmill
		BuildTrack(runAlong(shifted(square, 20, 0), 1)),
		BuildTrack(runAlong(outAndBack, 1)),
		BuildTrack(runAlong(shifted(square, 10, 0), 1)),
		BuildTrack(runAlong(reversed(square), 1.02)),
	}
	groups := GroupRoutes(tracks)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 (anticlockwise, clockwise): %+v", len(groups), groups)
	}
	if got := groups[0].Members; len(got) != 3 || got[0] != 0 || got[1] != 3 || got[2] != 5 {
		t.Errorf("anticlockwise members = %v, want [0 3 5]", got)
	}
	if groups[0].Representative != 5 {
		t.Errorf("representative = %d, want 5, the run between the other two", groups[0].Representative)
	}
	if got := groups[1].Members; len(got) != 2 || got[0] != 1 || got[1] != 6 {
		t.Errorf("clockwise members = %v, want [1 6]", got)
	}
}

func TestFrechetGivesUpPastLimit(t *testing.T) {
	a := [][2]float64{{0, 0}, {10, 0}}
	b := [][2]float64{{0, 100}, {10, 100}}
	if d := frechet(a, b, 50); !math.IsInf(d, 1) {
		t.Errorf("frechet = %v, want +Inf past the limit", d)
	}
	if d := frechet(a, b, 200); math.Abs(d-100) > 1e-9 {
		t.Errorf("frechet = %v, want 100", d)
	}
}
