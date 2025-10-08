package routes

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"gitea.deepak.science/deepak/trygo/internal/models"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
	"github.com/go-chi/chi/v5"
)

func newAuthRouter(m models.Model, tok tokens.Toker) http.Handler {
	router := chi.NewRouter()

	router.Post("/register", postUser(m))
	router.Post("/tokens", createTokenFunc(m, tok))
	return router
}

// type createUserResponse struct {
// 	Email string `json:"email"`
// }

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

		createUserResponse, err := m.CreateUser(ctx, &req)
		if err != nil {
			log.Printf("error with request body %v: %v", r.Body, err)
			serverError(w, err)
			return
		}
		log.Printf("created user: {%+v}", createUserResponse)

		w.WriteHeader(http.StatusCreated)
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(createUserResponse); err != nil {
			serverError(w, err)
		}
	}
}

type loginCreds struct {
	Email string `json:"email"`
	Password string `json:"password"`
}
type createdToken struct {
	Token string `json:"token"`
}

func createTokenFunc(m models.Model, tok tokens.Toker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var creds loginCreds
		err := dec.Decode(&creds)
		if err != nil {
			badRequestError(w, err)
			return
		}
		err = dec.Decode(&struct{}{})
		if err != io.EOF {
			badRequestError(w, err)
			return
		}

		user, err := m.VerifyUserByEmailPassword(ctx, creds.Email, creds.Password)
		if err != nil {
			// if models.IsInvalidLoginError(err) {
			// 	unauthorizedHandler(w, r)
			// 	return
			// }
			serverError(w, err)
			return

		}
		w.Header().Add("Content-Type", "application/json")
		response := &createdToken{Token: tok.EncodeUser(user)}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			serverError(w, err)
			return
		}
	}
}
