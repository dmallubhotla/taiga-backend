package routes

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/go-chi/chi/v5"
)

const jobListLimit = 100

func newJobRouter(m models.Model) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getJobsFunc(m))
	router.Get("/{jobid}", getJobFunc(m))
	router.Post("/{jobid}/retry", retryJobFunc(m))
	return router
}

func getJobsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		state := r.URL.Query().Get("state")
		switch state {
		case "", models.JobQueued, models.JobRunning, models.JobDone, models.JobDead:
		default:
			badRequestError(w, fmt.Errorf("unknown job state %q", state))
			return
		}

		jobs, err := m.Jobs(ctx, state, jobListLimit, userToken)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, jobs)
	}
}

func getJobFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		jobID, err := strconv.ParseInt(chi.URLParam(r, "jobid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		job, err := m.Job(ctx, int32(jobID), userToken)
		if models.IsNotFound(err) {
			notFoundHandler(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, job)
	}
}

func retryJobFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		jobID, err := strconv.ParseInt(chi.URLParam(r, "jobid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		job, err := m.RetryDeadJob(ctx, int32(jobID), time.Now(), userToken)
		switch {
		case errors.Is(err, models.ErrJobNotDead):
			conflictError(w, err)
		case models.IsNotFound(err):
			notFoundHandler(w, r)
		case err != nil:
			serverError(w, err)
		default:
			writeJSON(w, job)
		}
	}
}

func reprocessWorkoutFunc(m models.Model, fileRepo filerepo.FileRepo) http.HandlerFunc {
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

		workout, err := m.ReprocessWorkout(ctx, int32(workoutID), userToken, fileRepo)
		if err != nil {
			if !activityContentError(w, r, err) {
				serverError(w, err)
			}
			return
		}
		writeJSON(w, workout)
	}
}

type reprocessAllResponse struct {
	Enqueued int64 `json:"enqueued"`
}

// reprocessAllFunc queues every file-backed workout; progress shows up under GET /jobs.
func reprocessAllFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		n, err := m.EnqueueReprocessAll(ctx, time.Now(), userToken)
		if err != nil {
			serverError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, reprocessAllResponse{Enqueued: n})
	}
}
