package routes

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/models"
)

const maxWorkoutPage = 1000

// parseWindow reads ?from= and ?to= as a window of start times, from inclusive and to exclusive.
// Each is RFC3339 (the frontend sends the viewer's local midnight with its offset) or a bare date, taken as UTC midnight.
func parseWindow(q url.Values) (models.Window, error) {
	var w models.Window
	var err error
	if w.From, err = parseBound(q.Get("from")); err != nil {
		return w, fmt.Errorf("from: %w", err)
	}
	if w.To, err = parseBound(q.Get("to")); err != nil {
		return w, fmt.Errorf("to: %w", err)
	}
	if !w.From.IsZero() && !w.To.IsZero() && !w.To.After(w.From) {
		return w, fmt.Errorf("to must be after from")
	}
	return w, nil
}

func parseBound(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("use RFC3339 or YYYY-MM-DD, got %q", raw)
	}
	return t, nil
}

// parseWorkoutQuery adds ?limit= (1 to maxWorkoutPage; absent lists the whole window) and ?offset= to the window.
func parseWorkoutQuery(q url.Values) (models.WorkoutQuery, error) {
	window, err := parseWindow(q)
	if err != nil {
		return models.WorkoutQuery{}, err
	}
	query := models.WorkoutQuery{Window: window}
	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || limit < 1 || limit > maxWorkoutPage {
			return query, fmt.Errorf("limit must be an integer from 1 to %d", maxWorkoutPage)
		}
		query.Limit = int32(limit)
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || offset < 0 {
			return query, fmt.Errorf("offset must be a non-negative integer")
		}
		query.Offset = int32(offset)
	}
	return query, nil
}
