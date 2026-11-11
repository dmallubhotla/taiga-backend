package models

import (
	"context"
	"log"
	"math"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/jackc/pgx/v5/pgtype"
)

// Window limits a listing to workouts starting in [From, To); a zero end is open.
type Window struct {
	From, To time.Time
}

// Far-off bounds stand in for an open end, so one query serves both; UTC because SQLite compares timestamps as text.
var (
	windowStart = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
)

func (w Window) bounds() (from, to pgtype.Timestamptz) {
	f, t := windowStart, windowEnd
	if !w.From.IsZero() {
		f = w.From.UTC()
	}
	if !w.To.IsZero() {
		t = w.To.UTC()
	}
	return pgtype.Timestamptz{Time: f, Valid: true}, pgtype.Timestamptz{Time: t, Valid: true}
}

// WorkoutQuery pages through a window of the workout list, newest first; a zero Limit returns the rest of the window.
type WorkoutQuery struct {
	Window
	Limit, Offset int32
}

func (m *storeModel) Workouts(ctx context.Context, query WorkoutQuery, userToken *tokens.UserToken) ([]*Workout, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	from, to := query.bounds()
	limit := query.Limit
	if limit == 0 {
		limit = math.MaxInt32
	}
	workouts, err := querier.ListWorkoutsByUserInWindow(ctx, &db.ListWorkoutsByUserInWindowParams{
		UserID:     userToken.ID,
		WindowFrom: from,
		WindowTo:   to,
		PageLimit:  limit,
		PageOffset: query.Offset,
	})
	if err != nil {
		log.Printf("Could not get workouts: %v", err)
		return nil, err
	}
	return workouts, nil
}

// SportTotals sums one sport's counted workouts in a window.
type SportTotals = db.WorkoutTotalsByUserRow

type WorkoutTotals struct {
	Sports []*SportTotals `json:"sports"`
	// FirstStartTime is the earliest workout listed in the window, counted or not, so a client can offer the years there are; null when there's none.
	FirstStartTime *time.Time `json:"first_start_time"`
}

func (m *storeModel) WorkoutTotals(ctx context.Context, window Window, userToken *tokens.UserToken) (*WorkoutTotals, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	from, to := window.bounds()
	sports, err := querier.WorkoutTotalsByUser(ctx, &db.WorkoutTotalsByUserParams{UserID: userToken.ID, WindowFrom: from, WindowTo: to})
	if err != nil {
		log.Printf("Could not total workouts: %v", err)
		return nil, err
	}
	totals := &WorkoutTotals{Sports: sports}
	first, err := querier.FirstWorkoutStartInWindow(ctx, &db.FirstWorkoutStartInWindowParams{UserID: userToken.ID, WindowFrom: from, WindowTo: to})
	switch {
	case IsNotFound(err):
	case err != nil:
		log.Printf("Could not find the first workout: %v", err)
		return nil, err
	case first.Valid:
		totals.FirstStartTime = &first.Time
	}
	return totals, nil
}
