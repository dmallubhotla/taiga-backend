package tokens

import (
	"crypto/rsa"
	"fmt"
	"log"
	"os"

	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/models"
	"github.com/golang-jwt/jwt/v5"

	// "net/http"
	"time"
)

type Toker interface {
	EncodeUser(user *models.UserNoPassword) (string, error)
	DecodeTokenString(tokenString string) (*UserToken, error)
	// Authenticator(http.Handler) http.Handler
}

// type jwtToker struct {
// 	tokenAuth *jwtAuth.JWTAuth
// }

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
	jwt.RegisteredClaims
}

func (tok *rsaToker) EncodeUser(user *models.UserNoPassword) (string, error) {
	claims := standardClaims{
		user.Email,
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

	// TODO: Add database lookup to get user ID from email
	// For now, using placeholder ID
	return &UserToken{
		ID:    0, // TODO: lookup user ID from database using claims.Email
		Email: claims.Email,
	}, nil
}
