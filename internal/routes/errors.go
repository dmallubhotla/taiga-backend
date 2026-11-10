package routes

import (
	"errors"
	"log"
	"net/http"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

func serverError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	log.Printf("received error: {%v}", err)
	http.Error(w, http.StatusText(code), code)
}

func badRequestError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	log.Printf("received error: {%v}", err)
	http.Error(w, http.StatusText(code), code)
}

func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	code := http.StatusUnauthorized
	log.Print("Unauthorized error handled")
	http.Error(w, http.StatusText(code), code)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	code := http.StatusNotFound
	http.Error(w, http.StatusText(code), code)
}

// unprocessableError is for a well-formed request about content we can't use; the reason goes in the body so clients can show it.
func unprocessableError(w http.ResponseWriter, err error) {
	log.Printf("unprocessable: {%v}", err)
	http.Error(w, err.Error(), http.StatusUnprocessableEntity)
}

func conflictError(w http.ResponseWriter, err error) {
	log.Printf("conflict: {%v}", err)
	http.Error(w, err.Error(), http.StatusConflict)
}

// activityContentError maps errors from reading an activity file's content; it reports whether it handled err.
func activityContentError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case models.IsNotFound(err):
		notFoundHandler(w, r)
	case errors.Is(err, workouts.ErrUnreadableFile), errors.Is(err, workouts.ErrUnsupportedActivity),
		errors.Is(err, models.ErrNoFileContent), errors.Is(err, models.ErrNoActivityFile):
		unprocessableError(w, err)
	default:
		return false
	}
	return true
}
