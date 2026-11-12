package models

import (
	"cmp"
	"context"
	"errors"
	"log"
	"slices"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

// How a periodic total buckets workouts: weeks start on Monday, both at local midnight.
const (
	EveryWeek  = "week"
	EveryMonth = "month"
)

// MaxPeriods caps a periodic total, so a window of centuries can't ask for tens of thousands of empty weeks.
const MaxPeriods = 1000

var ErrTooManyPeriods = errors.New("the window spans too many periods")

// PeriodTotals sums each sport's counted workouts starting in one week or month.
type PeriodTotals struct {
	Start  time.Time      `json:"start"`
	Sports []*SportTotals `json:"sports"`
}

// periodStart is the local midnight starting the week or month t falls in.
func periodStart(t time.Time, every string, loc *time.Location) time.Time {
	t = t.In(loc)
	if every == EveryMonth {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
}

func nextPeriod(start time.Time, every string) time.Time {
	if every == EveryMonth {
		return start.AddDate(0, 1, 0)
	}
	return start.AddDate(0, 0, 7)
}

// PeriodicTotals lists every week or month from the window's start to its end, empty ones included; an open end stops at the first or last workout.
// A period straddling an edge of the window only sums the workouts inside it.
func (m *storeModel) PeriodicTotals(ctx context.Context, every string, window Window, loc *time.Location, userToken *tokens.UserToken) ([]*PeriodTotals, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	from, to := window.bounds()
	rows, err := querier.ListCountedWorkoutsInWindow(ctx, &db.ListCountedWorkoutsInWindowParams{UserID: userToken.ID, WindowFrom: from, WindowTo: to})
	if err != nil {
		log.Printf("Could not list counted workouts: %v", err)
		return nil, err
	}
	return bucketTotals(rows, every, window, loc)
}

func bucketTotals(rows []*db.ListCountedWorkoutsInWindowRow, every string, window Window, loc *time.Location) ([]*PeriodTotals, error) {
	first, last := window.From, window.To
	if !last.IsZero() {
		// To is exclusive, so a window ending on a period's first midnight doesn't include that period.
		last = last.Add(-time.Nanosecond)
	}
	if len(rows) > 0 {
		if first.IsZero() {
			first = rows[0].StartTime.Time
		}
		if last.IsZero() {
			last = rows[len(rows)-1].StartTime.Time
		}
	}
	if first.IsZero() || last.IsZero() {
		return []*PeriodTotals{}, nil
	}

	var periods []*PeriodTotals
	index := map[time.Time]*PeriodTotals{}
	end := periodStart(last, every, loc)
	for start := periodStart(first, every, loc); !start.After(end); start = nextPeriod(start, every) {
		if len(periods) == MaxPeriods {
			return nil, ErrTooManyPeriods
		}
		p := &PeriodTotals{Start: start, Sports: []*SportTotals{}}
		periods = append(periods, p)
		index[start] = p
	}
	for _, r := range rows {
		p := index[periodStart(r.StartTime.Time, every, loc)]
		i := slices.IndexFunc(p.Sports, func(s *SportTotals) bool { return s.Sport == r.Sport })
		if i < 0 {
			p.Sports = append(p.Sports, &SportTotals{Sport: r.Sport})
			i = len(p.Sports) - 1
		}
		s := p.Sports[i]
		s.Workouts++
		if r.DistanceMiles != nil {
			s.DistanceMiles += *r.DistanceMiles
		}
		if r.TimeSeconds != nil {
			s.TimeSeconds += *r.TimeSeconds
		}
	}
	for _, p := range periods {
		slices.SortFunc(p.Sports, func(a, b *SportTotals) int {
			return cmp.Or(cmp.Compare(b.Workouts, a.Workouts), cmp.Compare(a.Sport, b.Sport))
		})
	}
	return periods, nil
}

// YearBests is the best effort at each standard segment in one local calendar year.
type YearBests struct {
	Year    int                       `json:"year"`
	Efforts map[string]*SegmentEffort `json:"efforts"`
}

// YearlyBests ranks each year's efforts of one sport apart, newest year first, leaving out years with none.
func (m *storeModel) YearlyBests(ctx context.Context, sport string, loc *time.Location, userToken *tokens.UserToken) ([]*YearBests, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	rows, err := querier.ListSegmentEffortsBySport(ctx, &db.ListSegmentEffortsBySportParams{UserID: userToken.ID, Sport: sport})
	if err != nil {
		log.Printf("Could not list %s segments: %v", sport, err)
		return nil, err
	}
	byYear := map[int]*YearBests{}
	for _, r := range rows {
		effort := SegmentEffort(*r)
		year := effort.WorkoutStartTime.Time.In(loc).Year()
		y := byYear[year]
		if y == nil {
			y = &YearBests{Year: year, Efforts: map[string]*SegmentEffort{}}
			byYear[year] = y
		}
		if best := y.Efforts[effort.Name]; best == nil || better(&effort, best) {
			y.Efforts[effort.Name] = &effort
		}
	}
	years := make([]*YearBests, 0, len(byYear))
	for _, y := range byYear {
		years = append(years, y)
	}
	slices.SortFunc(years, func(a, b *YearBests) int { return cmp.Compare(b.Year, a.Year) })
	return years, nil
}

// better ranks as ListBestSegmentsByName does: faster over a distance, farther over a duration.
func better(a, b *SegmentEffort) bool {
	if a.Kind == workouts.KindDuration {
		return a.DistanceMeters > b.DistanceMeters
	}
	return a.DurationSeconds < b.DurationSeconds
}
