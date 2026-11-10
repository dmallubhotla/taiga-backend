package routes

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/go-chi/chi/v5"
)

func newAuthRouter(m models.Model, tok tokens.Toker, o options) http.Handler {
	router := chi.NewRouter()

	router.Get("/providers", getProvidersFunc(o))
	if o.registrationOpen {
		router.Post("/register", postUser(m))
	} else {
		router.Post("/register", registrationClosed)
	}
	router.Post("/tokens", createTokenFunc(m, tok))
	return router
}

// providersResponse tells the login page what to offer; OIDC providers (ROADMAP M1) will be listed here too.
type providersResponse struct {
	PasswordLogin    bool     `json:"password_login"`
	RegistrationOpen bool     `json:"registration_open"`
	Providers        []string `json:"providers"`
}

func getProvidersFunc(o options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, providersResponse{PasswordLogin: true, RegistrationOpen: o.registrationOpen, Providers: []string{}})
	}
}

func registrationClosed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "registration is closed; ask an admin for an account", http.StatusForbidden)
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
	Email    string `json:"email"`
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
		if errors.Is(err, models.ErrInvalidLogin) {
			unauthorizedHandler(w, r)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		userToken := &tokens.UserToken{
			ID:    user.ID,
			Email: user.Email,
		}
		token, err := tok.EncodeUser(userToken)
		if err != nil {
			log.Printf("Failed to encode user: %v", err)
			serverError(w, err)
			return
		}

		log.Printf("Issued token for user %d", user.ID)

		response := &createdToken{Token: token}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			serverError(w, err)
			return
		}
	}
}
