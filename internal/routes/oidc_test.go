package routes_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/oidc"
	"gitea.deepak.science/deepak/taiga/internal/routes"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

const frontendHost = "taiga.example.com"

type oidcAPI struct {
	t     *testing.T
	srv   *httptest.Server
	m     models.Model
	toker tokens.Toker
	idp   *testsupport.FakeOIDC
}

// newOIDCAPI serves the API with one provider, "fake", configured by edit.
func newOIDCAPI(t *testing.T, passwordLogin bool, edit func(*config.OIDCProvider)) *oidcAPI {
	t.Helper()
	m, repo, toker := testsupport.NewModel(t), testsupport.NewFileRepo(t), testsupport.NewToker(t)
	idp := testsupport.NewFakeOIDC(t)
	// The callback URL needs the server's address, which needs a handler first.
	var handler http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)

	p := config.OIDCProvider{Name: "fake", DisplayName: "Fake", Issuer: idp.Issuer(), ClientID: idp.ClientID}
	if edit != nil {
		edit(&p)
	}
	reg, err := oidc.New(t.Context(), config.AuthConfig{
		PublicBaseURL: "https://" + frontendHost,
		APIBaseURL:    srv.URL,
		OIDC:          []config.OIDCProvider{p},
	}, []byte("test flow key"))
	require.NoError(t, err)
	handler = routes.New(m, toker, repo, routes.WithOIDC(reg), routes.WithPasswordLogin(passwordLogin))
	return &oidcAPI{t: t, srv: srv, m: m, toker: toker, idp: idp}
}

// browser follows redirects with cookies, stopping at the frontend, or at stopAt's host too.
func (a *oidcAPI) browser(stopAt string) *http.Client {
	jar, err := cookiejar.New(nil)
	require.NoError(a.t, err)
	return &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Host == frontendHost || (stopAt != "" && strings.Contains(stopAt, req.URL.Host)) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

// landing is where the API sent the browser back to: the frontend route and its query.
type landing struct {
	route  string
	params url.Values
}

func (a *oidcAPI) land(resp *http.Response) landing {
	a.t.Helper()
	_ = resp.Body.Close()
	require.Equal(a.t, http.StatusFound, resp.StatusCode)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(a.t, err)
	require.Equal(a.t, frontendHost, loc.Host)
	route, query, _ := strings.Cut(loc.EscapedFragment(), "?")
	params, err := url.ParseQuery(query)
	require.NoError(a.t, err)
	return landing{route: route, params: params}
}

// signIn runs a whole sign-in as c and returns where it landed.
func (a *oidcAPI) signIn(c testsupport.IDClaims) landing {
	a.t.Helper()
	a.idp.SignInAs(c)
	resp, err := a.browser("").Get(a.srv.URL + "/auth/oidc/fake/login")
	require.NoError(a.t, err)
	return a.land(resp)
}

// signedIn signs in as c and returns the user the token is for.
func (a *oidcAPI) signedIn(c testsupport.IDClaims) *tokens.UserToken {
	a.t.Helper()
	l := a.signIn(c)
	require.Equal(a.t, "/login/callback", l.route)
	require.Empty(a.t, l.params.Get("error"))
	user, err := a.toker.DecodeTokenString(l.params.Get("token"))
	require.NoError(a.t, err)
	return user
}

func (a *oidcAPI) signInError(c testsupport.IDClaims) string {
	a.t.Helper()
	l := a.signIn(c)
	require.Equal(a.t, "/login/callback", l.route)
	assert.Empty(a.t, l.params.Get("token"))
	return l.params.Get("error")
}

func (a *oidcAPI) passwordUser(email string) *tokens.UserToken {
	a.t.Helper()
	resp, err := a.m.CreateUser(a.t.Context(), &models.CreateUserRequest{Email: email, DisplayName: email, Password: "password123"})
	require.NoError(a.t, err)
	return &tokens.UserToken{ID: resp.Id, Email: email}
}

// call makes an API request as user with the given client.
func (a *oidcAPI) call(c *http.Client, user *tokens.UserToken, method, path string) (int, []byte) {
	a.t.Helper()
	req, err := http.NewRequest(method, a.srv.URL+path, nil)
	require.NoError(a.t, err)
	if user != nil {
		token, err := a.toker.EncodeUser(user)
		require.NoError(a.t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	require.NoError(a.t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(a.t, err)
	return resp.StatusCode, body
}

// link links the next sign-in's identity to user, as the account page does, and returns where it landed.
func (a *oidcAPI) link(user *tokens.UserToken, c testsupport.IDClaims) landing {
	a.t.Helper()
	a.idp.SignInAs(c)
	browser := a.browser("")
	status, body := a.call(browser, user, http.MethodPost, "/auth/identities/fake")
	require.Equal(a.t, http.StatusOK, status, string(body))
	var started struct{ URL string }
	require.NoError(a.t, json.Unmarshal(body, &started))
	resp, err := browser.Get(started.URL)
	require.NoError(a.t, err)
	return a.land(resp)
}

func (a *oidcAPI) identities(user *tokens.UserToken) (password bool, ids []struct {
	ID          int32
	Provider    string
	EmailAtLink string `json:"email_at_link"`
}) {
	a.t.Helper()
	status, body := a.call(http.DefaultClient, user, http.MethodGet, "/auth/identities")
	require.Equal(a.t, http.StatusOK, status, string(body))
	var resp struct {
		Password   bool
		Identities []struct {
			ID          int32
			Provider    string
			EmailAtLink string `json:"email_at_link"`
		}
	}
	require.NoError(a.t, json.Unmarshal(body, &resp))
	return resp.Password, resp.Identities
}

var ada = testsupport.IDClaims{Subject: "ada-sub", Email: "ada@example.com", EmailVerified: true, Name: "Ada"}

func TestOIDCSignIn(t *testing.T) {
	a := newOIDCAPI(t, true, nil)

	status, body := a.call(http.DefaultClient, nil, http.MethodGet, "/auth/providers")
	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"password_login":true,"registration_open":true,"providers":[{"name":"fake","display_name":"Fake"}]}`, string(body))

	first := a.signedIn(ada)
	assert.Equal(t, "ada@example.com", first.Email)
	status, _ = a.call(http.DefaultClient, first, http.MethodGet, "/workouts")
	assert.Equal(t, http.StatusOK, status, "the token works on the rest of the API")

	again := a.signedIn(ada)
	assert.Equal(t, first.ID, again.ID, "the same identity signs in to the same account")

	renamed := ada
	renamed.Email = "ada@elsewhere.example.com"
	assert.Equal(t, first.ID, a.signedIn(renamed).ID, "an identity is its issuer and subject, not its email")

	status, _ = a.call(http.DefaultClient, nil, http.MethodGet, "/auth/oidc/nope/login")
	assert.Equal(t, http.StatusNotFound, status)

	// An account made through a provider has no password to sign in with: a 401, not a 500.
	status, _ = a.call(http.DefaultClient, nil, http.MethodPost, "/auth/tokens")
	assert.Equal(t, http.StatusBadRequest, status)
	resp, err := http.Post(a.srv.URL+"/auth/tokens", "application/json", strings.NewReader(`{"email":"ada@example.com","password":""}`))
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestOIDCRejectsBadCallbacks(t *testing.T) {
	a := newOIDCAPI(t, true, nil)

	// callbackURL starts a sign-in in browser and returns the provider's redirect back, without following it.
	callbackURL := func(browser *http.Client) *url.URL {
		a.idp.SignInAs(ada)
		resp, err := browser.Get(a.srv.URL + "/auth/oidc/fake/login")
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusFound, resp.StatusCode, "stopped at the provider")
		resp, err = (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(resp.Header.Get("Location"))
		require.NoError(t, err)
		_ = resp.Body.Close()
		back, err := url.Parse(resp.Header.Get("Location"))
		require.NoError(t, err)
		return back
	}
	finish := func(browser *http.Client, u *url.URL) landing {
		resp, err := browser.Get(u.String())
		require.NoError(t, err)
		return a.land(resp)
	}

	t.Run("a wrong state", func(t *testing.T) {
		browser := a.browser(a.idp.Server.URL)
		u := callbackURL(browser)
		q := u.Query()
		q.Set("state", "forged")
		u.RawQuery = q.Encode()
		assert.Equal(t, oidc.ErrFlow.Error(), finish(browser, u).params.Get("error"))
	})
	t.Run("no flow cookie", func(t *testing.T) {
		u := callbackURL(a.browser(a.idp.Server.URL))
		assert.Equal(t, oidc.ErrFlow.Error(), finish(a.browser(""), u).params.Get("error"), "another browser can't finish the flow")
	})
	t.Run("a replayed callback", func(t *testing.T) {
		browser := a.browser(a.idp.Server.URL)
		u := callbackURL(browser)
		require.NotEmpty(t, finish(browser, u).params.Get("token"))
		assert.Equal(t, oidc.ErrFlow.Error(), finish(browser, u).params.Get("error"), "the cookie is cleared")
	})

	failed := "signing in with Fake failed; try again"
	for name, c := range map[string]testsupport.IDClaims{
		"a wrong nonce":    {Subject: "s", Email: "s@example.com", Nonce: "replayed"},
		"a wrong audience": {Subject: "s", Email: "s@example.com", Audience: "another-client"},
		"an expired token": {Subject: "s", Email: "s@example.com", Expiry: time.Now().Add(-time.Hour)},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, failed, a.signInError(c))
		})
	}
	t.Run("the provider's error", func(t *testing.T) {
		assert.Contains(t, a.signInError(testsupport.IDClaims{Error: "access_denied"}), "access_denied")
	})
	_, err := a.m.VerifyUserByEmailPassword(t.Context(), "s@example.com", "")
	assert.ErrorIs(t, err, models.ErrInvalidLogin, "no failed sign-in made an account")
}

func TestOIDCSignupDisabled(t *testing.T) {
	a := newOIDCAPI(t, true, func(p *config.OIDCProvider) { p.AllowSignup = new(bool) })
	assert.Equal(t, models.ErrSignupClosed.Error(), a.signInError(ada))

	user := a.passwordUser("grace@example.com")
	a.link(user, ada)
	assert.Equal(t, user.ID, a.signedIn(ada).ID, "a linked identity still signs in")
}

func TestOIDCLinkByEmail(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run(fmt.Sprintf("link_by_verified_email=%v", on), func(t *testing.T) {
			a := newOIDCAPI(t, true, func(p *config.OIDCProvider) { p.LinkByVerifiedEmail = on })
			user := a.passwordUser("Ada@example.com")

			unverified := ada
			unverified.EmailVerified = false
			assert.Equal(t, models.ErrEmailTaken.Error(), a.signInError(unverified), "an unverified email never signs in to an account")

			if !on {
				assert.Equal(t, models.ErrEmailTaken.Error(), a.signInError(ada), "matching emails aren't linked unless the provider allows it")
				return
			}
			assert.Equal(t, user.ID, a.signedIn(ada).ID, "a verified email matching in any case links to the account")
			_, ids := a.identities(user)
			require.Len(t, ids, 1)
			assert.Equal(t, "fake", ids[0].Provider)
		})
	}
}

func TestOIDCNeedsAnEmailForANewAccount(t *testing.T) {
	a := newOIDCAPI(t, true, nil)
	assert.Equal(t, models.ErrNoEmail.Error(), a.signInError(testsupport.IDClaims{Subject: "no-email"}))
}

func TestOIDCLinkAndUnlink(t *testing.T) {
	a := newOIDCAPI(t, true, nil)
	grace := a.passwordUser("grace@example.com")

	l := a.link(grace, ada)
	assert.Equal(t, "/account", l.route, l.params.Encode())
	assert.Equal(t, "fake", l.params.Get("linked"))
	assert.Equal(t, grace.ID, a.signedIn(ada).ID, "the linked identity signs in to grace's account")
	assert.Equal(t, "/account", a.link(grace, ada).route, "linking it again is a no-op")

	password, ids := a.identities(grace)
	assert.True(t, password)
	require.Len(t, ids, 1)
	assert.Equal(t, "ada@example.com", ids[0].EmailAtLink)

	bob := a.passwordUser("bob@example.com")
	l = a.link(bob, ada)
	assert.Equal(t, models.ErrIdentityTaken.Error(), l.params.Get("error"))
	_, bobs := a.identities(bob)
	assert.Empty(t, bobs)

	status, _ := a.call(http.DefaultClient, bob, http.MethodDelete, fmt.Sprintf("/auth/identities/%d", ids[0].ID))
	assert.Equal(t, http.StatusNotFound, status, "bob can't unlink grace's identity")
	status, body := a.call(http.DefaultClient, grace, http.MethodDelete, fmt.Sprintf("/auth/identities/%d", ids[0].ID))
	assert.Equal(t, http.StatusNoContent, status, string(body))
	_, ids = a.identities(grace)
	assert.Empty(t, ids)

	// An account made through the provider has nothing else to sign in with.
	alan := a.signedIn(testsupport.IDClaims{Subject: "alan-sub", Email: "alan@example.com"})
	password, ids = a.identities(alan)
	assert.False(t, password)
	require.Len(t, ids, 1)
	status, body = a.call(http.DefaultClient, alan, http.MethodDelete, fmt.Sprintf("/auth/identities/%d", ids[0].ID))
	assert.Equal(t, http.StatusConflict, status)
	assert.Contains(t, string(body), "only way to sign in")

	status, _ = a.call(http.DefaultClient, alan, http.MethodPost, "/auth/identities/nope")
	assert.Equal(t, http.StatusNotFound, status)
}

func TestPasswordLoginOff(t *testing.T) {
	a := newOIDCAPI(t, false, nil)
	grace := a.passwordUser("grace@example.com")

	status, body := a.call(http.DefaultClient, nil, http.MethodGet, "/auth/providers")
	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"password_login":false,"registration_open":false,"providers":[{"name":"fake","display_name":"Fake"}]}`, string(body))
	for _, path := range []string{"/auth/tokens", "/auth/register"} {
		resp, err := http.Post(a.srv.URL+path, "application/json", strings.NewReader(`{"email":"grace@example.com","password":"password123"}`))
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, path)
	}

	a.link(grace, ada)
	password, ids := a.identities(grace)
	assert.False(t, password, "a password doesn't count while password login is off")
	status, _ = a.call(http.DefaultClient, grace, http.MethodDelete, fmt.Sprintf("/auth/identities/%d", ids[0].ID))
	assert.Equal(t, http.StatusConflict, status)
}

func TestOIDCBootChecks(t *testing.T) {
	idp := testsupport.NewFakeOIDC(t)
	good := config.OIDCProvider{Name: "fake", Issuer: idp.Issuer(), ClientID: idp.ClientID}
	for name, tc := range map[string]struct {
		cfg  config.AuthConfig
		want string
	}{
		"no public url":    {config.AuthConfig{OIDC: []config.OIDCProvider{good}}, "public_base_url"},
		"a bad name":       {config.AuthConfig{PublicBaseURL: "https://x.example.com", OIDC: []config.OIDCProvider{{Name: "Fake Co", Issuer: idp.Issuer(), ClientID: "c"}}}, "lowercase"},
		"a duplicate":      {config.AuthConfig{PublicBaseURL: "https://x.example.com", OIDC: []config.OIDCProvider{good, good}}, "twice"},
		"no discovery":     {config.AuthConfig{PublicBaseURL: "https://x.example.com", OIDC: []config.OIDCProvider{{Name: "gone", Issuer: idp.Issuer() + "/nothing", ClientID: "c"}}}, `"gone": discovery`},
		"a missing secret": {config.AuthConfig{PublicBaseURL: "https://x.example.com", OIDC: []config.OIDCProvider{{Name: "fake", Issuer: idp.Issuer(), ClientID: "c", ClientSecretFile: "/nonexistent"}}}, "client_secret_file"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := oidc.New(t.Context(), tc.cfg, []byte("k"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
	reg, err := oidc.New(t.Context(), config.AuthConfig{}, nil)
	require.NoError(t, err)
	assert.Empty(t, reg.Providers(), "no providers is fine")
}
