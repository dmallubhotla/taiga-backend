package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

func newWorkoutRouter(m models.Model, fileRepo filerepo.FileRepo) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getWorkoutsFunc(m))
	router.Get("/compare", compareWorkoutsFunc(m))
	router.Get("/totals", getWorkoutTotalsFunc(m))
	router.Get("/totals/{every}", getPeriodicTotalsFunc(m))
	router.Get("/{workoutid}", getWorkoutFunc(m))
	router.Get("/{workoutid}/segments", getWorkoutSegmentsFunc(m))
	router.Get("/{workoutid}/route", getWorkoutRouteFunc(m))
	router.With(middleware.Compress(5, "application/json")).Get("/{workoutid}/series", getWorkoutSeriesFunc(m))
	router.Post("/reprocess", reprocessAllFunc(m))
	router.Post("/{workoutid}/reprocess", reprocessWorkoutFunc(m, fileRepo))
	router.Put("/{workoutid}/corrections", putCorrectionsFunc(m, fileRepo))
	router.Post("/", postWorkoutFunc(m))
	router.Put("/{workoutid}", putWorkoutFunc(m))
	router.Delete("/{workoutid}", deleteWorkoutFunc(m))
	router.Post("/from-activity-file/{activityfileid}", postWorkoutFromActivityFileFunc(m, fileRepo))

	return router
}

func getWorkoutFunc(m models.Model) http.HandlerFunc {
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
		workoutID := int32(workoutID64)

		workout, err := m.Workout(ctx, workoutID, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(workout); err != nil {
			serverError(w, err)
		}
	}
}

func getWorkoutsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			log.Printf("Got error %v", err)
			unauthorizedHandler(w, r)
			return
		}

		query, err := parseWorkoutQuery(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		workouts, err := m.Workouts(ctx, query, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(workouts); err != nil {
			serverError(w, err)
		}
	}
}

func postWorkoutFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var workoutRequest struct {
			DistanceMiles  *float64 `json:"distance_miles"`
			TimeSeconds    *float64 `json:"time_seconds"`
			SpeedMph       *float64 `json:"speed_mph"`
			PaceMinPerMile *float64 `json:"pace_min_per_mile"`
			StartTime      *string  `json:"start_time"`
			EndTime        *string  `json:"end_time"`
			ActivityFileID *int32   `json:"activity_file_id"`
		}

		err = dec.Decode(&workoutRequest)
		if err != nil {
			badRequestError(w, err)
			return
		}

		workout := &models.Workout{
			DistanceMiles:  workoutRequest.DistanceMiles,
			TimeSeconds:    workoutRequest.TimeSeconds,
			SpeedMph:       workoutRequest.SpeedMph,
			PaceMinPerMile: workoutRequest.PaceMinPerMile,
			ActivityFileID: workoutRequest.ActivityFileID,
		}

		// Parse timestamps if provided
		if workoutRequest.StartTime != nil {
			startTime, err := time.Parse(time.RFC3339, *workoutRequest.StartTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid start_time format, use RFC3339: %w", err))
				return
			}
			workout.StartTime = pgtype.Timestamptz{Time: startTime.UTC(), Valid: true}
		}

		if workoutRequest.EndTime != nil {
			endTime, err := time.Parse(time.RFC3339, *workoutRequest.EndTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid end_time format, use RFC3339: %w", err))
				return
			}
			workout.EndTime = pgtype.Timestamptz{Time: endTime.UTC(), Valid: true}
		}

		log.Printf("Adding workout %v", workout)
		createdWorkout, err := m.AddWorkout(ctx, workout, userToken)
		if err != nil {
			log.Printf("Error adding workout! %v", err)
			serverError(w, err)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(createdWorkout); err != nil {
			serverError(w, err)
		}
	}
}

func putWorkoutFunc(m models.Model) http.HandlerFunc {
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
		workoutID := int32(workoutID64)

		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var workoutRequest struct {
			DistanceMiles  *float64 `json:"distance_miles"`
			TimeSeconds    *float64 `json:"time_seconds"`
			SpeedMph       *float64 `json:"speed_mph"`
			PaceMinPerMile *float64 `json:"pace_min_per_mile"`
			StartTime      *string  `json:"start_time"`
			EndTime        *string  `json:"end_time"`
			ActivityFileID *int32   `json:"activity_file_id"`
		}

		err = dec.Decode(&workoutRequest)
		if err != nil {
			badRequestError(w, err)
			return
		}

		// Get existing workout to merge with updates
		existingWorkout, err := m.Workout(ctx, workoutID, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		// Edits would be lost on the next reprocess; corrections that survive it go through PUT /workouts/{id}/corrections.
		if existingWorkout.ActivityFileID != nil {
			unprocessableError(w, models.ErrDerivedWorkout)
			return
		}
		if workoutRequest.ActivityFileID != nil {
			unprocessableError(w, errors.New("attach a file by creating the workout from it: POST /workouts/from-activity-file/{id}"))
			return
		}

		// Update fields that were provided
		if workoutRequest.DistanceMiles != nil {
			existingWorkout.DistanceMiles = workoutRequest.DistanceMiles
		}
		if workoutRequest.TimeSeconds != nil {
			existingWorkout.TimeSeconds = workoutRequest.TimeSeconds
		}
		if workoutRequest.SpeedMph != nil {
			existingWorkout.SpeedMph = workoutRequest.SpeedMph
		}
		if workoutRequest.PaceMinPerMile != nil {
			existingWorkout.PaceMinPerMile = workoutRequest.PaceMinPerMile
		}

		// Parse timestamps if provided
		if workoutRequest.StartTime != nil {
			startTime, err := time.Parse(time.RFC3339, *workoutRequest.StartTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid start_time format, use RFC3339: %w", err))
				return
			}
			existingWorkout.StartTime = pgtype.Timestamptz{Time: startTime.UTC(), Valid: true}
		}

		if workoutRequest.EndTime != nil {
			endTime, err := time.Parse(time.RFC3339, *workoutRequest.EndTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid end_time format, use RFC3339: %w", err))
				return
			}
			existingWorkout.EndTime = pgtype.Timestamptz{Time: endTime.UTC(), Valid: true}
		}

		log.Printf("Updating workout %v", workoutID)
		updatedWorkout, err := m.UpdateWorkout(ctx, workoutID, existingWorkout, userToken)
		if err != nil {
			log.Printf("Error updating workout! %v", err)
			serverError(w, err)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(updatedWorkout); err != nil {
			serverError(w, err)
		}
	}
}

func deleteWorkoutFunc(m models.Model) http.HandlerFunc {
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
		workoutID := int32(workoutID64)

		log.Printf("Deleting workout %v", workoutID)
		err = m.DeleteWorkout(ctx, workoutID, userToken)
		if err != nil {
			log.Printf("Error deleting workout! %v", err)
			serverError(w, err)
			return
		}

		queueRouteDetection(ctx, m, userToken)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postWorkoutFromActivityFileFunc(m models.Model, fileRepo filerepo.FileRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		activityFileID64, err := strconv.ParseInt(chi.URLParam(r, "activityfileid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		activityFileID := int32(activityFileID64)

		log.Printf("Creating workout from activity file %v", activityFileID)
		workout, created, err := m.CreateWorkoutFromActivityFile(ctx, activityFileID, userToken, fileRepo)
		if err != nil {
			if activityContentError(w, r, err) {
				return
			}
			log.Printf("Error creating workout from activity file! %v", err)
			serverError(w, err)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if created {
			queueRouteDetection(ctx, m, userToken)
			w.WriteHeader(http.StatusCreated)
		}
		if err := json.NewEncoder(w).Encode(workout); err != nil {
			serverError(w, err)
		}
	}
}

func getWorkoutSeriesFunc(m models.Model) http.HandlerFunc {
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

		series, err := m.WorkoutSeries(ctx, int32(workoutID64), userToken)
		switch {
		case models.IsNotFound(err):
			notFoundHandler(w, r)
		case errors.Is(err, models.ErrNoSeries):
			http.Error(w, err.Error(), http.StatusNotFound)
		case err != nil:
			serverError(w, err)
		default:
			// Stored already encoded, so serve the bytes as they are.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(series)
		}
	}
}

// maxExcludedReasonLength matches the excluded_reason column.
const maxExcludedReasonLength = 64

// putCorrectionsFunc replaces a file-backed workout's corrections and returns it reprocessed with them; a field left out or empty clears that correction.
func putCorrectionsFunc(m models.Model, fileRepo filerepo.FileRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		workoutID, err := strconv.ParseInt(chi.URLParam(r, "workoutid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var corrections models.Corrections
		if err := dec.Decode(&corrections); err != nil {
			badRequestError(w, err)
			return
		}
		corrections.Sport = strings.TrimSpace(corrections.Sport)
		corrections.ExcludedReason = strings.TrimSpace(corrections.ExcludedReason)
		if corrections.Sport != "" && !workouts.KnownSport(corrections.Sport) {
			http.Error(w, fmt.Sprintf("unknown sport %q: use a FIT sport name such as running, walking or cycling", corrections.Sport), http.StatusBadRequest)
			return
		}
		if utf8.RuneCountInString(corrections.ExcludedReason) > maxExcludedReasonLength {
			http.Error(w, fmt.Sprintf("excluded_reason must be at most %d characters", maxExcludedReasonLength), http.StatusBadRequest)
			return
		}

		workout, err := m.CorrectWorkout(ctx, int32(workoutID), corrections, userToken, fileRepo)
		if err != nil {
			if !activityContentError(w, r, err) {
				serverError(w, err)
			}
			return
		}
		// A new sport or exclusion can add the workout to a route or take it off one.
		queueRouteDetection(ctx, m, userToken)
		writeJSON(w, workout)
	}
}

// getWorkoutTotalsFunc sums each sport's counted workouts starting in ?from=/?to=; excluded workouts stay out.
func getWorkoutTotalsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		window, err := parseWindow(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		totals, err := m.WorkoutTotals(ctx, window, userToken)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, totals)
	}
}

// getPeriodicTotalsFunc sums the counted workouts per week or month of ?from=/?to=, at ?tz='s local midnights.
func getPeriodicTotalsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		every := chi.URLParam(r, "every")
		if every != models.EveryWeek && every != models.EveryMonth {
			notFoundHandler(w, r)
			return
		}
		window, err := parseWindow(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		loc, err := parseLocation(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		periods, err := m.PeriodicTotals(ctx, every, window, loc, userToken)
		if errors.Is(err, models.ErrTooManyPeriods) {
			http.Error(w, fmt.Sprintf("the window spans more than %d %ss", models.MaxPeriods, every), http.StatusBadRequest)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, periods)
	}
}
