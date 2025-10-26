package tokens

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"gitea.deepak.science/deepak/trygo/internal/config"
	"github.com/golang-jwt/jwt/v5"

	// "net/http"
	"time"
)

type Toker interface {
	EncodeUser(userToken *UserToken) (string, error)
	DecodeTokenString(tokenString string) (*UserToken, error)
	Authenticator(http.Handler) http.Handler
}

type rsaToker struct {
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
	issuer     string
	audience   string
}

// returns a new toker for the given secret key
func New(cfg config.Config) (Toker, error) {
	// TODO: Add issuer and audience to config
	// For now using app name as default values
	issuer := "trygo-app"
	audience := "trygo-users"

	if cfg.App.Environment != "" {
		issuer = "trygo-" + cfg.App.Environment
		audience = "trygo-" + cfg.App.Environment + "-users"
	}

	rawPrivateKey, err := os.ReadFile(cfg.Tokens.PrivateKeyPath)
	if err != nil {
		log.Printf("error reading private key: %v", err)
		return nil, err
	}
	rawPublicKey, err := os.ReadFile(cfg.Tokens.PublicKeyPath)
	if err != nil {
		log.Printf("error reading public key: %v", err)
		return nil, err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		log.Printf("error parsing private key: %v", err)
		return nil, err
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		log.Printf("error parsing public key: %v", err)
		return nil, err
	}

	return &rsaToker{
		publicKey:  publicKey,
		privateKey: privateKey,
		issuer:     issuer,
		audience:   audience,
	}, nil
}

type standardClaims struct {
	Email string `json:"email"`
	ID    int32  `json:"id"`
	jwt.RegisteredClaims
}

func (tok *rsaToker) EncodeUser(userToken *UserToken) (string, error) {
	claims := standardClaims{
		userToken.Email,
		userToken.ID,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    tok.issuer,
			Audience:  []string{tok.audience},
		},
	}
	log.Printf("Exporting claims %+v", claims)
	token := jwt.NewWithClaims(jwt.SigningMethodPS256, claims)

	signed, err := token.SignedString(tok.privateKey)
	if err != nil {
		log.Print(fmt.Errorf("error sadly: %w", err))
		return signed, err
	}

	return signed, nil

}

type UserToken struct {
	ID    int32
	Email string
}

func (tok *rsaToker) DecodeTokenString(tokenString string) (*UserToken, error) {

	token, err := jwt.ParseWithClaims(tokenString, &standardClaims{}, func(token *jwt.Token) (any, error) {
		return tok.publicKey, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodPS256.Alg()}),
		jwt.WithAudience(tok.audience),
		jwt.WithIssuer(tok.issuer),
		jwt.WithExpirationRequired(), // Requires and validates 'exp' claim
		jwt.WithIssuedAt(),           // Enables 'iat' claim validation
		jwt.WithTimeFunc(time.Now),   // Consistent time source for all validations
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(*standardClaims)
	if !ok {
		return nil, fmt.Errorf("failed to parse claims")
	}

	// Business logic validation (not handled by jwt.With* methods)
	if claims.Email == "" {
		return nil, fmt.Errorf("email claim is required")
	}

	return &UserToken{
		ID:    claims.ID,
		Email: claims.Email,
	}, nil
}

func tokenFromHeader(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	return ""
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	code := http.StatusUnauthorized
	http.Error(w, http.StatusText(code), code)
}

type contextKey struct {
	name string
}

var userTokenCtxKey = &contextKey{"UserToken"}

func (tok *rsaToker) Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := tokenFromHeader(r)
		if tokenString == "" {
			log.Print("No valid token found")
			unauthorized(w, r)
			return
		}

		userToken, err := tok.DecodeTokenString(tokenString)
		if err != nil {
			log.Printf("Error while verifying token: %v", err)
			unauthorized(w, r)
			return
		}

		// map our verified fields to our context for later
		log.Printf("Got user with id [%d], email [%s]", userToken.ID, userToken.Email)
		// this is the only place we should drop this boy on the context, because it's the only place it's authenticated
		// we can enforce this to some degree with a non-exported type
		ctx := context.WithValue(r.Context(), userTokenCtxKey, userToken)

		// authenticated
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserTokenFromContext(ctx context.Context) (*UserToken, error) {
	raw_token := ctx.Value(userTokenCtxKey)
	log.Println(raw_token)
	token, ok := ctx.Value(userTokenCtxKey).(*UserToken)
	if !ok {
		log.Printf("token: %v", token)
		log.Printf("ok: %v", ok)
		return nil, fmt.Errorf("Could not extract token from context")
	}
	return token, nil
}
