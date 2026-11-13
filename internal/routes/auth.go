package routes

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/oidc"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/go-chi/chi/v5"
)

func newAuthRouter(m models.Model, tok tokens.Toker, o options) http.Handler {
	router := chi.NewRouter()

	router.Get("/providers", getProvidersFunc(o))
	switch {
	case !o.passwordLogin:
		router.Post("/register", passwordLoginOff)
		router.Post("/tokens", passwordLoginOff)
	case o.registrationOpen:
		router.Post("/register", postUser(m))
		router.Post("/tokens", createTokenFunc(m, tok))
	default:
		router.Post("/register", registrationClosed)
		router.Post("/tokens", createTokenFunc(m, tok))
	}

	router.Get("/oidc/{provider}/login", oidcLogin(o.oidc))
	router.Get("/oidc/{provider}/callback", oidcCallback(m, tok, o.oidc))
	router.Group(func(r chi.Router) {
		r.Use(tok.Authenticator)
		r.Get("/identities", getIdentities(m, o))
		r.Post("/identities/{provider}", linkIdentity(o.oidc))
		r.Delete("/identities/{identityid}", unlinkIdentity(m, o))
	})
	return router
}

type providerInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// providersResponse tells the login page what to offer.
type providersResponse struct {
	PasswordLogin    bool           `json:"password_login"`
	RegistrationOpen bool           `json:"registration_open"`
	Providers        []providerInfo `json:"providers"`
}

func getProvidersFunc(o options) http.HandlerFunc {
	providers := []providerInfo{}
	for _, p := range o.oidc.Providers() {
		providers = append(providers, providerInfo{Name: p.Name, DisplayName: p.DisplayName})
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, providersResponse{
			PasswordLogin:    o.passwordLogin,
			RegistrationOpen: o.passwordLogin && o.registrationOpen,
			Providers:        providers,
		})
	}
}

func passwordLoginOff(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "password sign-in is off; sign in with a provider", http.StatusForbidden)
}

// oidcLogin sends the browser to the provider to sign in.
func oidcLogin(reg *oidc.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := reg.Provider(chi.URLParam(r, "provider"))
		if !ok {
			notFoundHandler(w, r)
			return
		}
		http.Redirect(w, r, reg.Begin(w, p, 0), http.StatusFound)
	}
}

// signInErrors are safe and useful to show the user as they are; anything else is logged and reported as a plain failure.
var signInErrors = []error{oidc.ErrFlow, models.ErrSignupClosed, models.ErrNoEmail, models.ErrEmailTaken, models.ErrIdentityTaken}

func signInMessage(p *oidc.Provider, err error) string {
	var pe *oidc.ProviderError
	if errors.As(err, &pe) {
		return pe.Error()
	}
	for _, known := range signInErrors {
		if errors.Is(err, known) {
			return err.Error()
		}
	}
	log.Printf("sign-in with %s failed: %v", p.Name, err)
	return "signing in with " + p.DisplayName + " failed; try again"
}

// oidcCallback finishes a provider sign-in or link and sends the browser back to the frontend: to #/login/callback with a token (in the fragment, which browsers don't send to servers or write to logs) or an error, or to #/account after linking.
func oidcCallback(m models.Model, tok tokens.Toker, reg *oidc.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := reg.Provider(chi.URLParam(r, "provider"))
		if !ok {
			notFoundHandler(w, r)
			return
		}
		back := func(route string, params url.Values) {
			http.Redirect(w, r, reg.FrontendURL("#/"+route+"?"+params.Encode()), http.StatusFound)
		}
		res, err := reg.Finish(r.Context(), w, r, p)
		if err != nil {
			back("login/callback", url.Values{"error": {signInMessage(p, err)}})
			return
		}
		id := models.Identity(res.Identity)

		if res.LinkUser != 0 {
			if err := m.LinkIdentity(r.Context(), id, &tokens.UserToken{ID: res.LinkUser}); err != nil {
				back("account", url.Values{"error": {signInMessage(p, err)}})
				return
			}
			log.Printf("Linked %s identity to user %d", p.Name, res.LinkUser)
			back("account", url.Values{"linked": {p.Name}})
			return
		}

		user, err := m.SignInWithIdentity(r.Context(), id, models.SignInPolicy{AllowSignup: p.AllowSignup, LinkByVerifiedEmail: p.LinkByVerifiedEmail})
		if err != nil {
			back("login/callback", url.Values{"error": {signInMessage(p, err)}})
			return
		}
		token, err := tok.EncodeUser(&tokens.UserToken{ID: user.ID, Email: user.Email})
		if err != nil {
			back("login/callback", url.Values{"error": {signInMessage(p, err)}})
			return
		}
		log.Printf("Issued token for user %d through %s", user.ID, p.Name)
		back("login/callback", url.Values{"token": {token}})
	}
}

type identityResponse struct {
	ID int32 `json:"id"`
	// The configured provider the identity came from; empty if it's no longer configured.
	Provider    string    `json:"provider"`
	DisplayName string    `json:"display_name"`
	Issuer      string    `json:"issuer"`
	EmailAtLink string    `json:"email_at_link"`
	CreatedAt   time.Time `json:"created_at"`
}

type identitiesResponse struct {
	// Whether the user can sign in with a password: they have one and password login is on.
	Password   bool               `json:"password"`
	Identities []identityResponse `json:"identities"`
}

func getIdentities(m models.Model, o options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			serverError(w, err)
			return
		}
		ids, err := m.Identities(r.Context(), userToken)
		if err != nil {
			serverError(w, err)
			return
		}
		out := identitiesResponse{Password: ids.HasPassword && o.passwordLogin, Identities: []identityResponse{}}
		for _, i := range ids.Identities {
			resp := identityResponse{ID: i.ID, DisplayName: i.Issuer, Issuer: i.Issuer, EmailAtLink: i.EmailAtLink, CreatedAt: i.CreatedAt}
			for _, p := range o.oidc.Providers() {
				if p.Issuer == i.Issuer {
					resp.Provider, resp.DisplayName = p.Name, p.DisplayName
				}
			}
			out.Identities = append(out.Identities, resp)
		}
		writeJSON(w, out)
	}
}

type linkResponse struct {
	URL string `json:"url"`
}

// linkIdentity starts a flow that links the provider's identity to the signed-in user; the frontend sends the browser to the returned URL.
// It can't redirect itself: a fetch carrying the bearer token doesn't navigate.
func linkIdentity(reg *oidc.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			serverError(w, err)
			return
		}
		p, ok := reg.Provider(chi.URLParam(r, "provider"))
		if !ok {
			notFoundHandler(w, r)
			return
		}
		writeJSON(w, linkResponse{URL: reg.Begin(w, p, userToken.ID)})
	}
}

func unlinkIdentity(m models.Model, o options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userToken, err := tokens.UserTokenFromContext(r.Context())
		if err != nil {
			serverError(w, err)
			return
		}
		id, err := strconv.ParseInt(chi.URLParam(r, "identityid"), 10, 32)
		if err != nil {
			badRequestError(w, err)
			return
		}
		err = m.UnlinkIdentity(r.Context(), int32(id), o.passwordLogin, userToken)
		switch {
		case models.IsNotFound(err):
			notFoundHandler(w, r)
		case errors.Is(err, models.ErrLastSignIn):
			conflictError(w, err)
		case err != nil:
			serverError(w, err)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
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
