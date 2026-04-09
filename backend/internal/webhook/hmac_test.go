package webhook_test

import (
	"testing"

	"github.com/quorant/quorant/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignPayload_KnownOutput(t *testing.T) {
	// Verified against: echo -n "hello world" | openssl dgst -sha256 -hmac "secret"
	payload := []byte("hello world")
	secret := "secret"

	sig := webhook.SignPayload(payload, secret)

	require.NotEmpty(t, sig)
	assert.Equal(t, "734cc62f32841568f45715aeb9f4d7891324e6d948e4c6c60c0621cdac48623a", sig)
}

func TestSignPayload_EmptyPayload(t *testing.T) {
	sig := webhook.SignPayload([]byte{}, "secret")
	require.NotEmpty(t, sig)
	// HMAC of empty payload is deterministic — should still be a valid 64-char hex string.
	assert.Len(t, sig, 64)
}

func TestVerifySignature_CorrectSignature(t *testing.T) {
	payload := []byte(`{"event":"test","data":"value"}`)
	secret := "my-webhook-secret"

	sig := webhook.SignPayload(payload, secret)
	ok := webhook.VerifySignature(payload, secret, sig)

	assert.True(t, ok)
}

func TestVerifySignature_WrongSignature(t *testing.T) {
	payload := []byte(`{"event":"test","data":"value"}`)
	secret := "my-webhook-secret"

	ok := webhook.VerifySignature(payload, secret, "deadbeef")

	assert.False(t, ok)
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	sig := webhook.SignPayload(payload, "correct-secret")

	ok := webhook.VerifySignature(payload, "wrong-secret", sig)

	assert.False(t, ok)
}

func TestVerifySignature_TamperedPayload(t *testing.T) {
	original := []byte(`{"amount":100}`)
	tampered := []byte(`{"amount":999}`)
	secret := "signing-key"

	sig := webhook.SignPayload(original, secret)
	ok := webhook.VerifySignature(tampered, secret, sig)

	assert.False(t, ok)
}
