package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/quorant/quorant/internal/platform/auth"
)

const testIssuer = "http://localhost:8085"
const testKeyID = "test-key-1"

// testKeyPair holds both the private JWK (for signing) and the public JWKS set
// (for validation), derived from the same RSA key.
type testKeyPair struct {
	privJWK jwk.Key
	keySet  jwk.Set
}

// generateTestKeySet generates an RSA key pair and returns a testKeyPair that
// can sign tokens and a JWKS set for validator construction.
func generateTestKeySet(t *testing.T) testKeyPair {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	// Create a JWK from the private key (carries kid + alg for signing).
	privJWK, err := jwk.FromRaw(privateKey)
	if err != nil {
		t.Fatalf("creating JWK from private key: %v", err)
	}
	if err := privJWK.Set(jwk.KeyIDKey, testKeyID); err != nil {
		t.Fatalf("setting key ID: %v", err)
	}
	if err := privJWK.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("setting algorithm: %v", err)
	}

	// Build a public-key-only JWKS set (what the validator will see).
	pubJWK, err := privJWK.PublicKey()
	if err != nil {
		t.Fatalf("extracting public JWK: %v", err)
	}

	keySet := jwk.NewSet()
	if err := keySet.AddKey(pubJWK); err != nil {
		t.Fatalf("adding public key to set: %v", err)
	}

	return testKeyPair{privJWK: privJWK, keySet: keySet}
}

// createTestToken builds and signs a JWT with the given extra claims.
func createTestToken(t *testing.T, kp testKeyPair, issuer string, exp time.Time, extraClaims map[string]any) string {
	t.Helper()

	builder := jwt.NewBuilder().
		Issuer(issuer).
		Subject("test-subject-id").
		IssuedAt(time.Now()).
		Expiration(exp)

	for k, v := range extraClaims {
		builder = builder.Claim(k, v)
	}

	token, err := builder.Build()
	if err != nil {
		t.Fatalf("building JWT: %v", err)
	}

	// Sign using the private JWK directly — the kid embedded in the JWK is
	// automatically included in the JWS protected header.
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, kp.privJWK))
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}

	return string(signed)
}

// newValidatorFromKeySet constructs a JWKSValidator directly from an in-memory
// key set — no HTTP server needed.
func newValidatorFromKeySet(t *testing.T, keySet jwk.Set, issuer string) auth.TokenValidator {
	t.Helper()
	v, err := auth.NewJWKSValidatorFromKeySet(keySet, issuer)
	if err != nil {
		t.Fatalf("creating JWKSValidator: %v", err)
	}
	return v
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestValidate_ValidToken(t *testing.T) {
	kp := generateTestKeySet(t)
	validator := newValidatorFromKeySet(t, kp.keySet, testIssuer)

	tokenStr := createTestToken(t, kp, testIssuer, time.Now().Add(time.Hour), nil)

	claims, err := validator.Validate(context.Background(), tokenStr)
	if err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
	if claims == nil {
		t.Fatal("Validate: expected non-nil claims")
	}
	if claims.Subject != "test-subject-id" {
		t.Errorf("Subject: got %q, want %q", claims.Subject, "test-subject-id")
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	kp := generateTestKeySet(t)
	validator := newValidatorFromKeySet(t, kp.keySet, testIssuer)

	// Expiry one hour in the past.
	tokenStr := createTestToken(t, kp, testIssuer, time.Now().Add(-time.Hour), nil)

	_, err := validator.Validate(context.Background(), tokenStr)
	if err == nil {
		t.Fatal("Validate: expected error for expired token, got nil")
	}
}

func TestValidate_WrongIssuer(t *testing.T) {
	kp := generateTestKeySet(t)
	// Validator expects testIssuer but token carries a different issuer.
	validator := newValidatorFromKeySet(t, kp.keySet, testIssuer)

	tokenStr := createTestToken(t, kp, "http://wrong-issuer.example.com", time.Now().Add(time.Hour), nil)

	_, err := validator.Validate(context.Background(), tokenStr)
	if err == nil {
		t.Fatal("Validate: expected error for wrong issuer, got nil")
	}
}

func TestValidate_InvalidSignature(t *testing.T) {
	kp := generateTestKeySet(t)
	validator := newValidatorFromKeySet(t, kp.keySet, testIssuer)

	tokenStr := createTestToken(t, kp, testIssuer, time.Now().Add(time.Hour), nil)

	// Corrupt the signature (last segment of the JWS compact serialisation).
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format: %d parts", len(parts))
	}
	parts[2] = "invalidsignatureXXXXXXXXXXXXXXXXXX"
	corruptedToken := strings.Join(parts, ".")

	_, err := validator.Validate(context.Background(), corruptedToken)
	if err == nil {
		t.Fatal("Validate: expected error for invalid signature, got nil")
	}
}

func TestValidate_ExtractsEmailAndName(t *testing.T) {
	kp := generateTestKeySet(t)
	validator := newValidatorFromKeySet(t, kp.keySet, testIssuer)

	tokenStr := createTestToken(t, kp, testIssuer, time.Now().Add(time.Hour), map[string]any{
		"email": "alice@example.com",
		"name":  "Alice Wonderland",
	})

	claims, err := validator.Validate(context.Background(), tokenStr)
	if err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", claims.Email, "alice@example.com")
	}
	if claims.Name != "Alice Wonderland" {
		t.Errorf("Name: got %q, want %q", claims.Name, "Alice Wonderland")
	}
}

// ---------------------------------------------------------------------------
// StaticValidator tests
// ---------------------------------------------------------------------------

func TestStaticValidator_ReturnsClaims(t *testing.T) {
	want := &auth.Claims{
		Subject: "static-sub",
		Email:   "static@example.com",
		Name:    "Static User",
	}

	v := auth.NewStaticValidator(want)
	got, err := v.Validate(context.Background(), "any-token-string")
	if err != nil {
		t.Fatalf("StaticValidator.Validate: unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("StaticValidator.Validate: got %+v, want %+v", got, want)
	}
}

func TestStaticValidator_ReturnsError(t *testing.T) {
	v := auth.NewStaticValidator(nil)
	v.Err = auth.ErrStaticValidatorErr

	_, err := v.Validate(context.Background(), "any-token-string")
	if err == nil {
		t.Fatal("StaticValidator.Validate: expected error, got nil")
	}
}
