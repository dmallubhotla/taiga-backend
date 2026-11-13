# Signing in with an OpenID Connect provider

taiga can offer sign-in through any OpenID Connect provider alongside (or instead of) passwords.
Endpoints come from the provider's discovery document, so authentik, Keycloak, Google and the rest are configuration, not code.
This page walks through authentik, then lists what any provider needs.

## How it works

1. The login page lists the providers from `GET /auth/providers` and links each to `/api/auth/oidc/{name}/login`.
2. The backend redirects to the provider with a `state`, a `nonce` and a PKCE challenge, kept in a signed, HttpOnly cookie scoped to the callback path, so the server holds no session.
3. The provider sends the browser back to `/api/auth/oidc/{name}/callback`. The backend checks `state`, exchanges the code, and verifies the ID token's signature, issuer, audience, expiry and `nonce`.
4. It finds the account linked to the token's issuer and subject (never its email), or links or creates one as the provider's settings allow, and redirects to the frontend's `#/login/callback` with the same taiga token a password sign-in gets. The token is in the URL fragment, which browsers don't send to servers or write to access logs.

An existing password account adds a provider from its account page (`POST /auth/identities/{name}`), and can unlink it there unless it's the only way left to sign in.

## authentik

1. In the admin interface, **Applications → Providers → Create → OAuth2/OpenID Provider**:
   - **Name**: `taiga`.
   - **Client type**: Confidential.
   - **Redirect URIs**: `https://taiga.example.com/api/auth/oidc/authentik/callback` (strict), with your frontend's origin and the `name` you give the provider in taiga's config.
   - **Signing key**: any certificate, so ID tokens are signed with RS256.
   - **Scopes**: the defaults include `openid`, `email` and `profile`, which is what taiga asks for.
   - **Subject mode**: the default, "Based on the User's hashed ID", is stable. Don't pick one based on email or username, which can change.
   Note the **Client ID** and **Client Secret**.
2. **Applications → Applications → Create**: name `taiga`, slug `taiga`, provider `taiga`. Bind users or groups to it to decide who may sign in.
3. The issuer is `https://auth.example.com/application/o/taiga/` (the slug, with the trailing slash); its discovery document is at `…/application/o/taiga/.well-known/openid-configuration`.
4. Put the client secret in a file the backend can read (a Kubernetes secret mounted as a file, say) and configure taiga:

   ```yaml
   auth:
     public_base_url: "https://taiga.example.com"
     oidc:
       - name: authentik
         display_name: "authentik"
         issuer: "https://auth.example.com/application/o/taiga/"
         client_id: "<client id>"
         client_secret_file: "/run/secrets/taiga-oidc-authentik"
   ```

   Or from the environment alone:

   ```sh
   TAIGA_AUTH_PUBLIC_BASE_URL=https://taiga.example.com
   TAIGA_AUTH_OIDC='[{"name":"authentik","display_name":"authentik","issuer":"https://auth.example.com/application/o/taiga/","client_id":"<client id>","client_secret_file":"/run/secrets/taiga-oidc-authentik"}]'
   ```

5. Restart the backend. It fetches each provider's discovery document at boot and refuses to start, naming the provider, if one can't be reached.

## Any provider

| Setting | |
|---|---|
| `name` | Lowercase letters, digits, `-` and `_`; it's in the callback path, so changing it changes the redirect URI. |
| `display_name` | The button label; defaults to `name`. |
| `issuer` | Exactly the `issuer` in the provider's discovery document, trailing slash included. ID tokens from any other issuer are rejected. |
| `client_id`, `client_secret_file` | A confidential client. Omit the secret file for a public client; PKCE is always used. |
| `scopes` | Defaults to `openid email profile`; `openid` is always added. A new account needs the `email` claim. |
| `allow_signup` | Default true. False means an identity no account has yet is refused, for invite-only setups; linking from a signed-in account still works. |
| `link_by_verified_email` | Default false. True signs an unknown identity in to the existing account with the same email (in any case), but only when the ID token has `email_verified: true`. Only turn it on for a provider whose emails you trust, since whoever controls the email at the provider gets the account. |

Register the redirect URI `{api_base_url}/auth/oidc/{name}/callback` with the provider.
`api_base_url` defaults to `public_base_url` + `/api`, the frontend's proxy to the backend; set it if the browser reaches the API somewhere else.

`auth.password_login: false` turns off `/auth/register` and `/auth/tokens` for a deployment that signs in only through providers. Accounts keep their passwords, and they work again if it's turned back on.

### Not handled

- Logging out at the provider. Signing out of taiga forgets taiga's token only.
- Refresh tokens. The taiga token lasts 2 hours. After that, signing in again is a redirect to the provider and straight back while its session lasts.
- Groups or roles from the provider.
