package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// ErrStaticValidatorErr is the sentinel error returned by StaticValidator when
// its Err field is set. Tests can use this value directly.
var ErrStaticValidatorErr = errors.New("static validator error")

// TokenValidator validates JWT tokens and extracts claims.
type TokenValidator interface {
	Validate(ctx context.Context, tokenString string) (*Claims, error)
}

// JWKSValidator validates JWTs using keys fetched from a JWKS endpoint.
type JWKSValidator struct {
	keySet jwk.Set
	issuer string
}

// NewJWKSValidator creates a validator that fetches keys from the given JWKS
// URL.  issuer is the expected "iss" claim value (e.g. "http://localhost:8085").
// jwksURL is the JWKS endpoint (e.g. "http://localhost:8085/oauth/v2/keys").
//
// In production consider using jwk.NewCache for automatic key rotation.
func NewJWKSValidator(ctx context.Context, jwksURL, issuer string) (*JWKSValidator, error) {
	keySet, err := jwk.Fetch(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	return &JWKSValidator{keySet: keySet, issuer: issuer}, nil
}

// NewJWKSValidatorFromKeySet creates a JWKSValidator using a pre-built jwk.Set.
// This is the recommended constructor for unit tests where no HTTP server is
// available.
func NewJWKSValidatorFromKeySet(keySet jwk.Set, issuer string) (*JWKSValidator, error) {
	if keySet == nil {
		return nil, errors.New("keySet must not be nil")
	}
	return &JWKSValidator{keySet: keySet, issuer: issuer}, nil
}

// Validate parses and validates a JWT token string, returning Claims on
// success.  It checks the signature against the JWKS key set, verifies expiry,
// and verifies the issuer claim.
func (v *JWKSValidator) Validate(_ context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKeySet(v.keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(v.issuer),
	)
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}

	claims := &Claims{
		Subject: token.Subject(),
	}

	if email, ok := token.Get("email"); ok {
		claims.Email, _ = email.(string)
	}
	if name, ok := token.Get("name"); ok {
		claims.Name, _ = name.(string)
	}

	return claims, nil
}

// ---------------------------------------------------------------------------
// StaticValidator – for use in tests that need a pre-canned TokenValidator.
// ---------------------------------------------------------------------------

// StaticValidator always returns the configured Claims (or Err).  It is
// intended for use in unit tests that need a TokenValidator but do not want to
// deal with real JWT signing.
type StaticValidator struct {
	Claims *Claims
	Err    error
}

// NewStaticValidator creates a StaticValidator that will return the given
// claims.  Set v.Err after construction to make Validate return an error.
func NewStaticValidator(claims *Claims) *StaticValidator {
	return &StaticValidator{Claims: claims}
}

// Validate implements TokenValidator.
func (v *StaticValidator) Validate(_ context.Context, _ string) (*Claims, error) {
	if v.Err != nil {
		return nil, v.Err
	}
	return v.Claims, nil
}
