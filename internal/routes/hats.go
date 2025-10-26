package routes

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"gitea.deepak.science/deepak/trygo/internal/models"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
	"github.com/go-chi/chi/v5"
)

func newHatRouter(m models.Model) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getHatsFunc(m))
	router.Get("/{hatid}", getHatFunc(m))
	router.Post("/", postHatFunc(m))

	return router
}

func getHatFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		hatID64, err := strconv.ParseInt(chi.URLParam(r, "hatid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		hatID := int32(hatID64)

		hat, err := m.Hat(ctx, hatID, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(hat); err != nil {
			serverError(w, err)
		}

	}
}

func getHatsFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			log.Printf("Got error %v", err)
			unauthorizedHandler(w, r)
			return
		}

		hats, err := m.Hats(ctx, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(hats); err != nil {
			serverError(w, err)
		}

	}
}

func postHatFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}
		log.Printf("token %v", userToken)

		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var h models.Hat
		err = dec.Decode(&h)
		if err != nil {
			badRequestError(w, err)
			return
		}

		log.Printf("Adding hat %v", &h)
		hat, err := m.AddHat(ctx, &h, userToken)
		if err != nil {
			log.Printf("Error adding hat! %v", err)
			serverError(w, err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(hat); err != nil {
			serverError(w, err)
		}

	}
}
