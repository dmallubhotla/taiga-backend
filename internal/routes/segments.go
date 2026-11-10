package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/go-chi/chi/v5"
)

const (
	defaultBestLimit = 10
	maxBestLimit     = 100
	maxCompareIDs    = 10
)

type standardDistanceResponse struct {
	Name   string  `json:"name"`
	Meters float64 `json:"meters"`
}

type compareResponse struct {
	// Standard distances at least one compared workout covers, shortest first.
	Distances []string                    `json:"distances"`
	Workouts  []*models.WorkoutComparison `json:"workouts"`
}

func newSegmentRouter(m models.Model) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getStandardDistancesFunc())
	router.Get("/{name}/best", getBestSegmentsFunc(m))
	return router
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		serverError(w, err)
	}
}

func getStandardDistancesFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		distances := make([]standardDistanceResponse, 0, len(workouts.StandardDistances))
		for _, d := range workouts.StandardDistances {
			distances = append(distances, standardDistanceResponse{Name: d.Name, Meters: d.Meters})
		}
		writeJSON(w, distances)
	}
}

func getBestSegmentsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		name := chi.URLParam(r, "name")
		if _, ok := workouts.StandardDistance(name); !ok {
			notFoundHandler(w, r)
			return
		}

		limit := int64(defaultBestLimit)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.ParseInt(raw, 10, 32)
			if err != nil || limit < 1 || limit > maxBestLimit {
				badRequestError(w, fmt.Errorf("limit must be an integer from 1 to %d", maxBestLimit))
				return
			}
		}

		efforts, err := m.BestSegments(ctx, name, int32(limit), userToken)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, efforts)
	}
}

func getWorkoutSegmentsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		workoutID64, err := strconv.ParseInt(chi.URLParam(r, "workoutid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		segments, err := m.WorkoutSegments(ctx, int32(workoutID64), userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		writeJSON(w, segments)
	}
}

func parseCompareIDs(raw string) ([]int32, error) {
	if raw == "" {
		return nil, fmt.Errorf("ids is required, e.g. ?ids=1,2")
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxCompareIDs {
		return nil, fmt.Errorf("at most %d workouts can be compared", maxCompareIDs)
	}
	ids := make([]int32, 0, len(parts))
	seen := make(map[int32]bool, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid workout id %q", p)
		}
		if !seen[int32(id)] {
			seen[int32(id)] = true
			ids = append(ids, int32(id))
		}
	}
	return ids, nil
}

func compareWorkoutsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		ids, err := parseCompareIDs(r.URL.Query().Get("ids"))
		if err != nil {
			badRequestError(w, err)
			return
		}

		comparisons, err := m.CompareWorkouts(ctx, ids, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		resp := compareResponse{Distances: []string{}, Workouts: comparisons}
		for _, d := range workouts.StandardDistances {
			for _, c := range comparisons {
				if _, ok := c.Segments[d.Name]; ok {
					resp.Distances = append(resp.Distances, d.Name)
					break
				}
			}
		}
		writeJSON(w, resp)
	}
}
