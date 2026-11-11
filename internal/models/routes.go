package models

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/jackc/pgx/v5/pgtype"
)

const JobDetectRoutes = "detect_routes"

// ErrNoRoute means the workout isn't on any route: it has no GPS, or no other run matches it yet.
var ErrNoRoute = errors.New("workout is not on a route")

// RouteSummary is a route with figures over its runs.
type RouteSummary struct {
	ID   int32   `json:"id"`
	Name *string `json:"name"`
	// RepresentativeWorkoutID is the run whose track stands for the route; null briefly after that run is deleted.
	RepresentativeWorkoutID *int32 `json:"representative_workout_id"`
	RunCount                int    `json:"run_count"`
	// Sport is shared by every run on the route.
	Sport string `json:"sport"`
	// DistanceMiles is the representative run's.
	DistanceMiles   *float64           `json:"distance_miles"`
	BestTimeSeconds *float64           `json:"best_time_seconds"`
	BestWorkoutID   *int32             `json:"best_workout_id"`
	LatestStartTime pgtype.Timestamptz `json:"latest_start_time"`
}

// RouteDetail is a route with its runs, newest first, and the representative's track.
type RouteDetail struct {
	RouteSummary
	Workouts []*Workout `json:"workouts"`
	// [lat, lon] pairs; empty if the representative run is gone.
	Track [][2]float64 `json:"track"`
}

func summarize(route *db.Route, runs []*Workout) *RouteSummary {
	s := &RouteSummary{ID: route.ID, Name: route.Name, RepresentativeWorkoutID: route.RepresentativeWorkoutID, RunCount: len(runs)}
	for _, w := range runs {
		if s.RepresentativeWorkoutID != nil && w.ID == *s.RepresentativeWorkoutID {
			s.DistanceMiles = w.DistanceMiles
			s.Sport = w.Sport
		}
		if w.TimeSeconds != nil && (s.BestTimeSeconds == nil || *w.TimeSeconds < *s.BestTimeSeconds) {
			s.BestTimeSeconds, s.BestWorkoutID = w.TimeSeconds, &w.ID
		}
		if !s.LatestStartTime.Valid || w.StartTime.Time.After(s.LatestStartTime.Time) {
			s.LatestStartTime = w.StartTime
		}
	}
	return s
}

// routeRuns groups the user's routed workouts by route, each newest first.
func routeRuns(ctx context.Context, q db.Querier, userToken *tokens.UserToken) (map[int32][]*Workout, error) {
	memberships, err := q.ListWorkoutRoutesByUser(ctx, userToken.ID)
	if err != nil {
		return nil, err
	}
	routeOf := make(map[int32]int32, len(memberships))
	for _, wr := range memberships {
		routeOf[wr.WorkoutID] = wr.RouteID
	}
	// Already newest first.
	all, err := q.ListWorkoutsByUser(ctx, userToken.ID)
	if err != nil {
		return nil, err
	}
	runs := make(map[int32][]*Workout)
	for _, w := range all {
		if r, ok := routeOf[w.ID]; ok {
			runs[r] = append(runs[r], w)
		}
	}
	return runs, nil
}

// Routes lists the user's routes, most recently run first.
func (m *storeModel) Routes(ctx context.Context, userToken *tokens.UserToken) ([]*RouteSummary, error) {
	q, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	routes, err := q.ListRoutesByUser(ctx, userToken.ID)
	if err != nil {
		return nil, err
	}
	runs, err := routeRuns(ctx, q, userToken)
	if err != nil {
		return nil, err
	}
	out := make([]*RouteSummary, 0, len(routes))
	for _, r := range routes {
		out = append(out, summarize(r, runs[r.ID]))
	}
	slices.SortStableFunc(out, func(a, b *RouteSummary) int {
		return b.LatestStartTime.Time.Compare(a.LatestStartTime.Time)
	})
	return out, nil
}

func (m *storeModel) Route(ctx context.Context, id int32, userToken *tokens.UserToken) (*RouteDetail, error) {
	q, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	route, err := q.GetRoute(ctx, &db.GetRouteParams{ID: id, UserID: userToken.ID})
	if err != nil {
		return nil, err
	}
	runs, err := routeRuns(ctx, q, userToken)
	if err != nil {
		return nil, err
	}
	detail := &RouteDetail{RouteSummary: *summarize(route, runs[id]), Workouts: runs[id], Track: [][2]float64{}}
	if detail.Workouts == nil {
		detail.Workouts = []*Workout{}
	}
	if route.RepresentativeWorkoutID != nil {
		raw, err := q.GetWorkoutTrack(ctx, &db.GetWorkoutTrackParams{WorkoutID: *route.RepresentativeWorkoutID, UserID: userToken.ID})
		if err != nil && !IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			var track workouts.Track
			if err := json.Unmarshal(raw, &track); err != nil {
				return nil, fmt.Errorf("decoding track of workout %d: %w", *route.RepresentativeWorkoutID, err)
			}
			detail.Track = track.Points
		}
	}
	return detail, nil
}

// RenameRoute sets the route's name, or clears it with nil.
func (m *storeModel) RenameRoute(ctx context.Context, id int32, name *string, userToken *tokens.UserToken) (*RouteSummary, error) {
	q, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	route, err := q.RenameRoute(ctx, &db.RenameRouteParams{ID: id, UserID: userToken.ID, Name: name})
	if err != nil {
		return nil, err
	}
	runs, err := routeRuns(ctx, q, userToken)
	if err != nil {
		return nil, err
	}
	return summarize(route, runs[id]), nil
}

// WorkoutRoute is the route the workout is on; ErrNoRoute if none, not-found if the workout isn't the user's.
func (m *storeModel) WorkoutRoute(ctx context.Context, workoutID int32, userToken *tokens.UserToken) (*RouteSummary, error) {
	if _, err := m.Workout(ctx, workoutID, userToken); err != nil {
		return nil, err
	}
	q, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	routeID, err := q.GetWorkoutRouteID(ctx, &db.GetWorkoutRouteIDParams{WorkoutID: workoutID, UserID: userToken.ID})
	if IsNotFound(err) {
		return nil, ErrNoRoute
	}
	if err != nil {
		return nil, err
	}
	detail, err := m.Route(ctx, routeID, userToken)
	if err != nil {
		return nil, err
	}
	return &detail.RouteSummary, nil
}

// EnqueueDetectRoutes queues route detection for the user, unless it's already pending.
func (m *storeModel) EnqueueDetectRoutes(ctx context.Context, now time.Time, userToken *tokens.UserToken) error {
	q, err := m.store.GetQuerier()
	if err != nil {
		return err
	}
	return q.EnqueueDetectRoutes(ctx, &db.EnqueueDetectRoutesParams{UserID: userToken.ID, RunAt: timestamptz(now)})
}

// DetectRoutes regroups all the user's tracked runs into routes from scratch.
// Each new group keeps the id, and so the name, of the old route it shares the most runs with; old routes nothing claims are deleted.
func (m *storeModel) DetectRoutes(ctx context.Context, userToken *tokens.UserToken) error {
	return m.store.InTx(ctx, func(q db.Querier) error {
		rows, err := q.ListWorkoutTracksByUser(ctx, userToken.ID)
		if err != nil {
			return err
		}
		tracks := make([]*workouts.Track, len(rows))
		for i, row := range rows {
			var t workouts.Track
			if err := json.Unmarshal(row.Track, &t); err != nil {
				log.Printf("Skipping undecodable track of workout %d: %v", row.WorkoutID, err)
				continue
			}
			tracks[i] = &t
		}
		// Routes never mix sports, so each sport is grouped on its own; members stay indices into rows.
		var groups []workouts.RouteGroup
		for _, sport := range workouts.ScoredSports {
			var idx []int
			var sub []*workouts.Track
			for i, row := range rows {
				if row.Sport == sport {
					idx = append(idx, i)
					sub = append(sub, tracks[i])
				}
			}
			for _, g := range workouts.GroupRoutes(sub) {
				members := make([]int, len(g.Members))
				for k, m := range g.Members {
					members[k] = idx[m]
				}
				groups = append(groups, workouts.RouteGroup{Members: members, Representative: idx[g.Representative]})
			}
		}

		old, err := q.ListWorkoutRoutesByUser(ctx, userToken.ID)
		if err != nil {
			return err
		}
		oldRoute := make(map[int32]int32, len(old))
		for _, wr := range old {
			oldRoute[wr.WorkoutID] = wr.RouteID
		}
		existing, err := q.ListRoutesByUser(ctx, userToken.ID)
		if err != nil {
			return err
		}

		// Bigger groups pick first, so a route that split keeps its id on the larger part.
		slices.SortStableFunc(groups, func(a, b workouts.RouteGroup) int { return cmp.Compare(len(b.Members), len(a.Members)) })
		claimed := make(map[int32]bool)
		if err := q.DeleteWorkoutRoutesByUser(ctx, userToken.ID); err != nil {
			return err
		}
		for _, g := range groups {
			overlap := make(map[int32]int)
			for _, i := range g.Members {
				if r, ok := oldRoute[rows[i].WorkoutID]; ok && !claimed[r] {
					overlap[r]++
				}
			}
			rep := rows[g.Representative].WorkoutID
			var routeID int32
			for r, n := range overlap {
				if routeID == 0 || n > overlap[routeID] || (n == overlap[routeID] && r < routeID) {
					routeID = r
				}
			}
			if routeID != 0 {
				claimed[routeID] = true
				if err := q.SetRouteRepresentative(ctx, &db.SetRouteRepresentativeParams{ID: routeID, UserID: userToken.ID, RepresentativeWorkoutID: &rep}); err != nil {
					return err
				}
			} else {
				route, err := q.CreateRoute(ctx, &db.CreateRouteParams{UserID: userToken.ID, RepresentativeWorkoutID: &rep})
				if err != nil {
					return err
				}
				routeID = route.ID
				claimed[routeID] = true
			}
			for _, i := range g.Members {
				if err := q.AddWorkoutRoute(ctx, &db.AddWorkoutRouteParams{WorkoutID: rows[i].WorkoutID, RouteID: routeID, UserID: userToken.ID}); err != nil {
					return err
				}
			}
		}
		for _, r := range existing {
			if !claimed[r.ID] {
				if err := q.DeleteRoute(ctx, &db.DeleteRouteParams{ID: r.ID, UserID: userToken.ID}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// storeTrack replaces the workout's track, removing it for a run without GPS or a sport that doesn't get routes.
func storeTrack(ctx context.Context, q db.Querier, workoutID int32, run *workouts.Run, userToken *tokens.UserToken) error {
	var track *workouts.Track
	if workouts.Scored(run.Summary.Sport) {
		track = workouts.BuildTrack(run)
	}
	if track == nil {
		return q.DeleteWorkoutTrack(ctx, &db.DeleteWorkoutTrackParams{WorkoutID: workoutID, UserID: userToken.ID})
	}
	raw, err := json.Marshal(track)
	if err != nil {
		return fmt.Errorf("encoding track for workout %d: %w", workoutID, err)
	}
	return q.UpsertWorkoutTrack(ctx, &db.UpsertWorkoutTrackParams{WorkoutID: workoutID, UserID: userToken.ID, Track: raw})
}
