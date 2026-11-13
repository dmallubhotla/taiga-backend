package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

// ErrSignupClosed means an unknown identity at a provider with allow_signup off.
var ErrSignupClosed = errors.New("there's no account for this sign-in, and this provider doesn't create them; ask an admin, or sign in another way and link it from your account page")

// ErrNoEmail means the provider gave no email, which a new account needs.
var ErrNoEmail = errors.New("the provider didn't share an email address, which a new account needs; allow the email scope")

// ErrEmailTaken means a new account would reuse an existing account's email.
var ErrEmailTaken = errors.New("an account with this email already exists; sign in with it and link this provider from your account page")

// ErrIdentityTaken means the identity already signs in to a different account.
var ErrIdentityTaken = errors.New("this sign-in is already linked to another account")

// ErrLastSignIn means unlinking would leave the user no way to sign in.
var ErrLastSignIn = errors.New("this is your only way to sign in; link another or set a password first")

// Identity is who a provider's ID token says signed in; Issuer and Subject identify it, the rest is only for a new account or for linking by email.
type Identity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// SignInPolicy is what a provider may do for an identity no account has yet.
type SignInPolicy struct {
	AllowSignup         bool
	LinkByVerifiedEmail bool
}

type LinkedIdentity struct {
	ID          int32     `json:"id"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	EmailAtLink string    `json:"email_at_link"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserIdentities struct {
	HasPassword bool              `json:"has_password"`
	Identities  []*LinkedIdentity `json:"identities"`
}

// SignInWithIdentity finds the identity's account, or links or creates one as the policy allows.
func (m *storeModel) SignInWithIdentity(ctx context.Context, id Identity, policy SignInPolicy) (*UserNoPassword, error) {
	var user *UserNoPassword
	err := m.store.InTx(ctx, func(q db.Querier) error {
		found, err := q.GetIdentityUser(ctx, &db.GetIdentityUserParams{Issuer: id.Issuer, Subject: id.Subject})
		if err == nil {
			user = &UserNoPassword{ID: found.ID, Email: found.Email, DisplayName: found.DisplayName}
			return nil
		}
		if !IsNotFound(err) {
			return err
		}

		var existing *db.GetUserByEmailRow
		if id.Email != "" {
			found, err := q.GetUserByEmail(ctx, id.Email)
			switch {
			case err == nil:
				existing = found
			case !IsNotFound(err):
				return err
			}
		}
		// Only a verified email may sign in to an existing account, or anyone who can set an email at some provider could take it over.
		if existing != nil && policy.LinkByVerifiedEmail && id.EmailVerified {
			user = &UserNoPassword{ID: existing.ID, Email: existing.Email, DisplayName: existing.DisplayName}
			return link(ctx, q, existing.ID, id)
		}
		if !policy.AllowSignup {
			return ErrSignupClosed
		}
		if id.Email == "" {
			return ErrNoEmail
		}
		if existing != nil {
			return ErrEmailTaken
		}

		name := id.Name
		if name == "" {
			name = id.Email
		}
		created, err := q.CreateUser(ctx, &db.CreateUserParams{Email: id.Email, DisplayName: name})
		if err != nil {
			return err
		}
		user = &UserNoPassword{ID: created.ID, Email: created.Email, DisplayName: created.DisplayName}
		return link(ctx, q, created.ID, id)
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// LinkIdentity lets the identity sign in to the user's account; linking one already on it is a no-op.
func (m *storeModel) LinkIdentity(ctx context.Context, id Identity, userToken *tokens.UserToken) error {
	return m.store.InTx(ctx, func(q db.Querier) error {
		found, err := q.GetIdentityUser(ctx, &db.GetIdentityUserParams{Issuer: id.Issuer, Subject: id.Subject})
		if err == nil {
			if found.ID == userToken.ID {
				return nil
			}
			return ErrIdentityTaken
		}
		if !IsNotFound(err) {
			return err
		}
		return link(ctx, q, userToken.ID, id)
	})
}

func link(ctx context.Context, q db.Querier, userID int32, id Identity) error {
	_, err := q.CreateIdentity(ctx, &db.CreateIdentityParams{UserID: userID, Issuer: id.Issuer, Subject: id.Subject, EmailAtLink: id.Email})
	return err
}

func (m *storeModel) Identities(ctx context.Context, userToken *tokens.UserToken) (*UserIdentities, error) {
	q, err := m.store.GetQuerier()
	if err != nil {
		return nil, err
	}
	return identities(ctx, q, userToken.ID)
}

func identities(ctx context.Context, q db.Querier, userID int32) (*UserIdentities, error) {
	hasPassword, err := q.UserHasPassword(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListIdentitiesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &UserIdentities{HasPassword: hasPassword, Identities: make([]*LinkedIdentity, 0, len(rows))}
	for _, r := range rows {
		out.Identities = append(out.Identities, &LinkedIdentity{ID: r.ID, Issuer: r.Issuer, Subject: r.Subject, EmailAtLink: r.EmailAtLink, CreatedAt: r.CreatedAt.Time})
	}
	return out, nil
}

// UnlinkIdentity removes one of the user's identities, unless nothing else would sign them in; a password counts only while password login is on.
func (m *storeModel) UnlinkIdentity(ctx context.Context, id int32, passwordLogin bool, userToken *tokens.UserToken) error {
	return m.store.InTx(ctx, func(q db.Querier) error {
		ids, err := identities(ctx, q, userToken.ID)
		if err != nil {
			return err
		}
		mine := false
		for _, i := range ids.Identities {
			mine = mine || i.ID == id
		}
		if !mine {
			return sql.ErrNoRows
		}
		passwordSignIn := ids.HasPassword && passwordLogin
		if len(ids.Identities) == 1 && !passwordSignIn {
			return ErrLastSignIn
		}
		_, err = q.DeleteIdentity(ctx, &db.DeleteIdentityParams{ID: id, UserID: userToken.ID})
		return err
	})
}
