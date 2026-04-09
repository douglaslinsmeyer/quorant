package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignPayload generates an HMAC-SHA256 signature for a payload using the given secret.
// The signature is returned as a hex string.
func SignPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks if a signature matches the expected HMAC-SHA256 of the payload.
// Uses constant-time comparison to prevent timing attacks.
func VerifySignature(payload []byte, secret, signature string) bool {
	expected := SignPayload(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
