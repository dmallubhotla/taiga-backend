// Package oidc signs users in through OpenID Connect providers: the authorization code flow with PKCE, its state kept in a signed cookie so the server stays stateless.
package oidc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"gitea.deepak.science/deepak/taiga/internal/config"
)

const (
	cookieName = "taiga_oidc"
	// Long enough to type a password and pass a second factor at the provider.
	flowTTL = 10 * time.Minute
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ErrFlow means the callback doesn't belong to a flow this server started, or the flow expired; the user should just start again.
var ErrFlow = errors.New("the sign-in expired or didn't start here; try again")

// Provider is one configured OpenID Connect provider.
type Provider struct {
	Name                string
	DisplayName         string
	Issuer              string
	AllowSignup         bool
	LinkByVerifiedEmail bool

	oauth    oauth2.Config
	verifier *gooidc.IDTokenVerifier
	// The callback's path, which scopes the flow cookie.
	callbackPath string
	secure       bool
}

// Registry holds the configured providers.
type Registry struct {
	providers  []*Provider
	publicBase string
	key        []byte
	now        func() time.Time
}

// New discovers every configured provider's endpoints, failing if any can't be reached, so a misconfigured provider stops the server at boot rather than at someone's sign-in.
// key signs the flow cookie.
func New(ctx context.Context, cfg config.AuthConfig, key []byte) (*Registry, error) {
	r := &Registry{publicBase: strings.TrimSuffix(cfg.PublicBaseURL, "/"), key: key, now: time.Now}
	if len(cfg.OIDC) == 0 {
		return r, nil
	}
	if r.publicBase == "" {
		return nil, errors.New("auth.public_base_url must be set to use OIDC providers")
	}
	api, err := url.Parse(cfg.APIBase())
	if err != nil {
		return nil, fmt.Errorf("auth.api_base_url: %w", err)
	}
	seen := map[string]bool{}
	for _, pc := range cfg.OIDC {
		if !validName.MatchString(pc.Name) {
			return nil, fmt.Errorf("OIDC provider name %q must be lowercase letters, digits, - and _", pc.Name)
		}
		if seen[pc.Name] {
			return nil, fmt.Errorf("OIDC provider %q is configured twice", pc.Name)
		}
		seen[pc.Name] = true
		p, err := newProvider(ctx, pc, api)
		if err != nil {
			return nil, fmt.Errorf("OIDC provider %q: %w", pc.Name, err)
		}
		r.providers = append(r.providers, p)
	}
	return r, nil
}

func newProvider(ctx context.Context, pc config.OIDCProvider, api *url.URL) (*Provider, error) {
	if pc.Issuer == "" || pc.ClientID == "" {
		return nil, errors.New("issuer and client_id are required")
	}
	secret := ""
	if pc.ClientSecretFile != "" {
		raw, err := os.ReadFile(pc.ClientSecretFile)
		if err != nil {
			return nil, fmt.Errorf("reading client_secret_file: %w", err)
		}
		secret = strings.TrimSpace(string(raw))
	}
	discovered, err := gooidc.NewProvider(ctx, pc.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	scopes := pc.Scopes
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "email", "profile"}
	}
	if !slices.Contains(scopes, gooidc.ScopeOpenID) {
		scopes = append([]string{gooidc.ScopeOpenID}, scopes...)
	}
	callback := api.JoinPath("auth", "oidc", pc.Name, "callback")
	display := pc.DisplayName
	if display == "" {
		display = pc.Name
	}
	return &Provider{
		Name:                pc.Name,
		DisplayName:         display,
		Issuer:              pc.Issuer,
		AllowSignup:         pc.AllowSignup == nil || *pc.AllowSignup,
		LinkByVerifiedEmail: pc.LinkByVerifiedEmail,
		oauth: oauth2.Config{
			ClientID:     pc.ClientID,
			ClientSecret: secret,
			Endpoint:     discovered.Endpoint(),
			RedirectURL:  callback.String(),
			Scopes:       scopes,
		},
		verifier:     discovered.Verifier(&gooidc.Config{ClientID: pc.ClientID}),
		callbackPath: "/" + strings.TrimPrefix(callback.Path, "/"),
		secure:       callback.Scheme == "https",
	}, nil
}

// Providers lists the configured providers in config order.
func (r *Registry) Providers() []*Provider { return r.providers }

func (r *Registry) Provider(name string) (*Provider, bool) {
	i := slices.IndexFunc(r.providers, func(p *Provider) bool { return p.Name == name })
	if i < 0 {
		return nil, false
	}
	return r.providers[i], true
}

// FrontendURL is a frontend route, e.g. "#/login/callback?token=...", as an absolute URL.
func (r *Registry) FrontendURL(fragment string) string {
	return r.publicBase + "/" + fragment
}

// flow is what the callback needs to check that it answers this browser's request.
type flow struct {
	Provider string `json:"p"`
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	// The account to link the identity to; 0 for a sign-in.
	LinkUser int32 `json:"u,omitempty"`
	Expires  int64 `json:"e"`
}

// Begin starts a flow at the provider: it sets the flow cookie on w and returns the URL to send the browser to.
// linkUser is the signed-in user an identity is being linked to, or 0 to sign in.
func (r *Registry) Begin(w http.ResponseWriter, p *Provider, linkUser int32) string {
	f := flow{
		Provider: p.Name,
		State:    random(),
		Nonce:    random(),
		Verifier: oauth2.GenerateVerifier(),
		LinkUser: linkUser,
		Expires:  r.now().Add(flowTTL).Unix(),
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    r.sign(f),
		Path:     p.callbackPath,
		MaxAge:   int(flowTTL.Seconds()),
		HttpOnly: true,
		Secure:   p.secure,
		// Lax still sends it on the provider's top-level redirect back.
		SameSite: http.SameSiteLaxMode,
	})
	return p.oauth.AuthCodeURL(f.State, gooidc.Nonce(f.Nonce), oauth2.S256ChallengeOption(f.Verifier))
}

// Identity is who the provider's ID token says signed in.
type Identity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Result is a finished flow: who signed in, and the account to link them to (0 to sign in).
type Result struct {
	Identity Identity
	LinkUser int32
}

// ProviderError is an error the provider sent back instead of a code, e.g. the user declined.
type ProviderError struct {
	Code        string
	Description string
}

func (e *ProviderError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("the provider said %s: %s", e.Code, e.Description)
	}
	return "the provider said " + e.Code
}

// Finish checks the callback against its flow cookie, exchanges the code, and verifies the ID token's signature, issuer, audience, expiry and nonce.
// It clears the cookie either way, so a callback can't be replayed.
func (r *Registry) Finish(ctx context.Context, w http.ResponseWriter, req *http.Request, p *Provider) (*Result, error) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: p.callbackPath, MaxAge: -1, HttpOnly: true, Secure: p.secure, SameSite: http.SameSiteLaxMode})
	cookie, err := req.Cookie(cookieName)
	if err != nil {
		return nil, ErrFlow
	}
	f, err := r.verify(cookie.Value)
	if err != nil || f.Provider != p.Name || r.now().Unix() > f.Expires {
		return nil, ErrFlow
	}
	q := req.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.State)) != 1 {
		return nil, ErrFlow
	}
	if code := q.Get("error"); code != "" {
		return nil, &ProviderError{Code: code, Description: q.Get("error_description")}
	}

	token, err := p.oauth.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(f.Verifier))
	if err != nil {
		return nil, fmt.Errorf("exchanging the code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("the token response has no id_token")
	}
	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("verifying the ID token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(f.Nonce)) != 1 {
		return nil, errors.New("the ID token's nonce doesn't match")
	}
	var claims struct {
		Email             string `json:"email"`
		EmailVerified     *bool  `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("reading the ID token's claims: %w", err)
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	return &Result{
		Identity: Identity{
			Issuer:        idToken.Issuer,
			Subject:       idToken.Subject,
			Email:         claims.Email,
			EmailVerified: claims.EmailVerified != nil && *claims.EmailVerified,
			Name:          name,
		},
		LinkUser: f.LinkUser,
	}, nil
}

func (r *Registry) sign(f flow) string {
	body, _ := json.Marshal(f)
	payload := base64.RawURLEncoding.EncodeToString(body)
	return payload + "." + base64.RawURLEncoding.EncodeToString(r.mac(payload))
}

func (r *Registry) verify(value string) (*flow, error) {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return nil, ErrFlow
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, r.mac(payload)) {
		return nil, ErrFlow
	}
	body, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrFlow
	}
	var f flow
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, ErrFlow
	}
	return &f, nil
}

func (r *Registry) mac(payload string) []byte {
	h := hmac.New(sha256.New, r.key)
	h.Write([]byte(payload))
	return h.Sum(nil)
}

func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
