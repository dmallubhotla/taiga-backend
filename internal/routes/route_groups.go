package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/go-chi/chi/v5"
)

const maxRouteNameLength = 100

// Route groups are served under /routes; this file is named apart from routes.go, which is the HTTP router.
func newRouteRouter(m models.Model) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getRoutesFunc(m))
	router.Get("/{routeid}", getRouteFunc(m))
	router.Put("/{routeid}", putRouteFunc(m))
	return router
}

// queueRouteDetection regroups the user's routes in the background after their tracks change.
// A failure only delays the regrouping until the next change, so it's logged rather than failing the request.
func queueRouteDetection(ctx context.Context, m models.Model, userToken *tokens.UserToken) {
	if err := m.EnqueueDetectRoutes(ctx, time.Now(), userToken); err != nil {
		log.Printf("Could not queue route detection for user %d: %v", userToken.ID, err)
	}
}

func getRoutesFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		window, err := parseWindow(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		routes, err := m.Routes(r.Context(), window, userToken)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, routes)
	}
}

func routeID(r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "routeid"), 10, 32)
	return int32(id), err == nil
}

func getRouteFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		id, ok := routeID(r)
		if !ok {
			notFoundHandler(w, r)
			return
		}
		window, err := parseWindow(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		route, err := m.Route(r.Context(), id, window, userToken)
		if models.IsNotFound(err) {
			notFoundHandler(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, route)
	}
}

type putRouteRequest struct {
	// Name is trimmed; empty clears it.
	Name *string `json:"name"`
}

func putRouteFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		id, ok := routeID(r)
		if !ok {
			notFoundHandler(w, r)
			return
		}
		var req putRouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequestError(w, fmt.Errorf("invalid JSON: %w", err))
			return
		}
		var name *string
		if req.Name != nil {
			if trimmed := strings.TrimSpace(*req.Name); trimmed != "" {
				name = &trimmed
			}
		}
		if name != nil && len(*name) > maxRouteNameLength {
			badRequestError(w, fmt.Errorf("name must be at most %d characters", maxRouteNameLength))
			return
		}
		route, err := m.RenameRoute(r.Context(), id, name, userToken)
		if models.IsNotFound(err) {
			notFoundHandler(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, route)
	}
}

func getWorkoutRouteFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		workoutID, err := strconv.ParseInt(chi.URLParam(r, "workoutid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		route, err := m.WorkoutRoute(r.Context(), int32(workoutID), userToken)
		if errors.Is(err, models.ErrNoRoute) || models.IsNotFound(err) {
			notFoundHandler(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, route)
	}
}
