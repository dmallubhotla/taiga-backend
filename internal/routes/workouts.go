package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func newWorkoutRouter(m models.Model, fileRepo filerepo.FileRepo) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getWorkoutsFunc(m))
	router.Get("/{workoutid}", getWorkoutFunc(m))
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

		workouts, err := m.Workouts(ctx, userToken)
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
			workout.StartTime = pgtype.Timestamptz{Time: startTime, Valid: true}
		}

		if workoutRequest.EndTime != nil {
			endTime, err := time.Parse(time.RFC3339, *workoutRequest.EndTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid end_time format, use RFC3339: %w", err))
				return
			}
			workout.EndTime = pgtype.Timestamptz{Time: endTime, Valid: true}
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
		if workoutRequest.ActivityFileID != nil {
			existingWorkout.ActivityFileID = workoutRequest.ActivityFileID
		}

		// Parse timestamps if provided
		if workoutRequest.StartTime != nil {
			startTime, err := time.Parse(time.RFC3339, *workoutRequest.StartTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid start_time format, use RFC3339: %w", err))
				return
			}
			existingWorkout.StartTime = pgtype.Timestamptz{Time: startTime, Valid: true}
		}

		if workoutRequest.EndTime != nil {
			endTime, err := time.Parse(time.RFC3339, *workoutRequest.EndTime)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid end_time format, use RFC3339: %w", err))
				return
			}
			existingWorkout.EndTime = pgtype.Timestamptz{Time: endTime, Valid: true}
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
		workout, err := m.CreateWorkoutFromActivityFile(ctx, activityFileID, userToken, fileRepo)
		if err != nil {
			log.Printf("Error creating workout from activity file! %v", err)
			serverError(w, err)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(workout); err != nil {
			serverError(w, err)
		}
	}
}
