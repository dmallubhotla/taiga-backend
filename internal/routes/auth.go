package routes

import (
	"encoding/json"
	"log"
	"net/http"

	"gitea.deepak.science/deepak/trygo/internal/models"
	"github.com/go-chi/chi/v5"
)

func NewAuthRouter(m models.Model) http.Handler {
	router := chi.NewRouter()

	return router
}

type createUserResponse struct {
	Username string `json:"username"`
}

func postUser(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// golang we always need this
		ctx := r.Context()

		// limit body size
		r.Body = http.MaxBytesReader(w, r.Body, 1024)

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req models.CreateUserRequest
		err := dec.Decode(&req)
		if err != nil {
			badRequestError(w, err)
			return
		}

		userId, err := m.CreateUser(ctx, &req)
		if err != nil {
			serverError(w, err)
			return
		}
		log.Printf("created user: {%v}", userId)

		response := &createUserResponse{
			Username: "username",
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			serverError(w, err)
		}
	}
}
