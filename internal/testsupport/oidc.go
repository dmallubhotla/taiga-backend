package testsupport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

// FakeOIDC is an in-process OpenID Connect provider: discovery, an authorize endpoint that signs in at once, a token endpoint that checks PKCE, and JWKS.
type FakeOIDC struct {
	Server   *httptest.Server
	ClientID string

	mu    sync.Mutex
	next  IDClaims
	codes map[string]authRequest
	key   *rsa.PrivateKey
}

// IDClaims is the ID token the next sign-in gets; the zero values of the overrides mean a correct token.
type IDClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string

	Audience string
	Expiry   time.Time
	Nonce    string
	// Error makes the authorize endpoint send this OAuth error back instead of a code.
	Error string
}

type authRequest struct {
	nonce     string
	challenge string
	claims    IDClaims
}

func NewFakeOIDC(t *testing.T) *FakeOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &FakeOIDC{ClientID: "taiga-test", codes: map[string]authRequest{}, key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /authorize", f.authorize)
	mux.HandleFunc("POST /token", f.token)
	mux.HandleFunc("GET /jwks", f.jwks)
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Server.Close)
	return f
}

func (f *FakeOIDC) Issuer() string { return f.Server.URL }

// SignInAs sets the ID token the next sign-in gets.
func (f *FakeOIDC) SignInAs(c IDClaims) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next = c
}

func (f *FakeOIDC) discovery(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                f.Issuer(),
		"authorization_endpoint":                f.Issuer() + "/authorize",
		"token_endpoint":                        f.Issuer() + "/token",
		"jwks_uri":                              f.Issuer() + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *FakeOIDC) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || q.Get("client_id") != f.ClientID || q.Get("code_challenge_method") != "S256" {
		http.Error(w, "bad authorize request", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	claims := f.next
	code := rand.Text()
	f.codes[code] = authRequest{nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), claims: claims}
	f.mu.Unlock()

	back := url.Values{"state": {q.Get("state")}}
	if claims.Error != "" {
		back.Set("error", claims.Error)
	} else {
		back.Set("code", code)
	}
	redirect.RawQuery = back.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (f *FakeOIDC) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	req, ok := f.codes[r.PostForm.Get("code")]
	delete(f.codes, r.PostForm.Get("code"))
	f.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != req.challenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}

	c := req.claims
	aud, exp, nonce := f.ClientID, time.Now().Add(time.Hour), req.nonce
	if c.Audience != "" {
		aud = c.Audience
	}
	if !c.Expiry.IsZero() {
		exp = c.Expiry
	}
	if c.Nonce != "" {
		nonce = c.Nonce
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, (&jose.SignerOptions{}).WithHeader("kid", "fake"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idToken, err := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer:   f.Issuer(),
		Subject:  c.Subject,
		Audience: jwt.Audience{aud},
		Expiry:   jwt.NewNumericDate(exp),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}).Claims(map[string]any{
		"nonce":          nonce,
		"email":          c.Email,
		"email_verified": c.EmailVerified,
		"name":           c.Name,
	}).Serialize()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
}

func (f *FakeOIDC) jwks(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "fake", Algorithm: "RS256", Use: "sig"}}})
}
