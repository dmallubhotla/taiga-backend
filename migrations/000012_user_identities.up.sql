-- Sign-ins at an OpenID Connect provider linked to a user; a sign-in is looked up by (issuer, subject) only, never by email.
CREATE TABLE user_identities (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  issuer TEXT NOT NULL,
  subject TEXT NOT NULL,
  -- The provider's email when the identity was linked, to tell identities apart; never used to sign in.
  email_at_link TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
  UNIQUE (issuer, subject)
);

CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);
