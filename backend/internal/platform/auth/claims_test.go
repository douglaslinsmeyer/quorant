package auth_test

import (
	"context"
	"testing"

	"github.com/quorant/quorant/internal/platform/auth"
)

func TestClaimsFields(t *testing.T) {
	claims := &auth.Claims{
		Subject: "zitadel-user-id-123",
		Email:   "user@example.com",
		Name:    "Jane Doe",
	}

	if claims.Subject != "zitadel-user-id-123" {
		t.Errorf("Subject: got %q, want %q", claims.Subject, "zitadel-user-id-123")
	}
	if claims.Email != "user@example.com" {
		t.Errorf("Email: got %q, want %q", claims.Email, "user@example.com")
	}
	if claims.Name != "Jane Doe" {
		t.Errorf("Name: got %q, want %q", claims.Name, "Jane Doe")
	}
}

func TestWithClaims_ClaimsFromContext_RoundTrip(t *testing.T) {
	original := &auth.Claims{
		Subject: "zitadel-user-id-456",
		Email:   "another@example.com",
		Name:    "John Smith",
	}

	ctx := auth.WithClaims(context.Background(), original)

	retrieved, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("ClaimsFromContext: expected ok=true, got false")
	}
	if retrieved == nil {
		t.Fatal("ClaimsFromContext: expected non-nil claims")
	}

	if retrieved.Subject != original.Subject {
		t.Errorf("Subject: got %q, want %q", retrieved.Subject, original.Subject)
	}
	if retrieved.Email != original.Email {
		t.Errorf("Email: got %q, want %q", retrieved.Email, original.Email)
	}
	if retrieved.Name != original.Name {
		t.Errorf("Name: got %q, want %q", retrieved.Name, original.Name)
	}
}

func TestClaimsFromContext_EmptyContext(t *testing.T) {
	claims, ok := auth.ClaimsFromContext(context.Background())

	if ok {
		t.Error("ClaimsFromContext on empty context: expected ok=false, got true")
	}
	if claims != nil {
		t.Errorf("ClaimsFromContext on empty context: expected nil claims, got %+v", claims)
	}
}

func TestWithClaims_NilClaims(t *testing.T) {
	ctx := auth.WithClaims(context.Background(), nil)

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		t.Error("ClaimsFromContext after storing nil: expected ok=true (value was stored), got false")
	}
	if claims != nil {
		t.Errorf("ClaimsFromContext after storing nil: expected nil claims, got %+v", claims)
	}
}

func TestWithClaims_DoesNotMutateParentContext(t *testing.T) {
	parent := context.Background()
	claims := &auth.Claims{Subject: "user-789"}

	child := auth.WithClaims(parent, claims)

	// Parent should remain unaffected.
	_, ok := auth.ClaimsFromContext(parent)
	if ok {
		t.Error("parent context should not have claims after WithClaims on child")
	}

	// Child should have them.
	retrieved, ok := auth.ClaimsFromContext(child)
	if !ok {
		t.Fatal("child context should have claims")
	}
	if retrieved.Subject != claims.Subject {
		t.Errorf("Subject: got %q, want %q", retrieved.Subject, claims.Subject)
	}
}
