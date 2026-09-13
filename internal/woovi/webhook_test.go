package woovi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func TestParseWebhookEvent(t *testing.T) {
	t.Run("registration test ping is ignored", func(t *testing.T) {
		evt, err := ParseWebhookEvent([]byte(`{"evento":"teste_webhook"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != EventIgnored {
			t.Errorf("Type = %q, want %q", evt.Type, EventIgnored)
		}
	})

	t.Run("body with no event key is ignored", func(t *testing.T) {
		evt, err := ParseWebhookEvent([]byte(`{"foo":"bar"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != EventIgnored {
			t.Errorf("Type = %q, want %q", evt.Type, EventIgnored)
		}
	})

	t.Run("charge completed", func(t *testing.T) {
		body := []byte(`{"event":"OPENPIX:CHARGE_COMPLETED","charge":{"correlationID":"driver-topup-abc","value":5000,"paidAt":"2026-01-01T10:00:00Z"}}`)
		evt, err := ParseWebhookEvent(body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != EventPaid {
			t.Errorf("Type = %q, want %q", evt.Type, EventPaid)
		}
		if evt.CorrelationID != "driver-topup-abc" {
			t.Errorf("CorrelationID = %q, want %q", evt.CorrelationID, "driver-topup-abc")
		}
		if evt.ValueCents != 5000 {
			t.Errorf("ValueCents = %d, want 5000", evt.ValueCents)
		}
		wantKey := "OPENPIX:CHARGE_COMPLETED:driver-topup-abc"
		if evt.EventKey != wantKey {
			t.Errorf("EventKey = %q, want %q", evt.EventKey, wantKey)
		}
		if evt.PaidAt == nil {
			t.Error("PaidAt should be set")
		}
	})

	t.Run("charge expired", func(t *testing.T) {
		body := []byte(`{"event":"OPENPIX:CHARGE_EXPIRED","charge":{"correlationID":"passenger-topup-xyz"}}`)
		evt, err := ParseWebhookEvent(body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if evt.Type != EventExpired {
			t.Errorf("Type = %q, want %q", evt.Type, EventExpired)
		}
	})

	t.Run("same correlation ID, different event types produce different keys", func(t *testing.T) {
		paid, _ := ParseWebhookEvent([]byte(`{"event":"OPENPIX:CHARGE_COMPLETED","charge":{"correlationID":"c1"}}`))
		expired, _ := ParseWebhookEvent([]byte(`{"event":"OPENPIX:CHARGE_EXPIRED","charge":{"correlationID":"c1"}}`))
		if paid.EventKey == expired.EventKey {
			t.Errorf("expected distinct event keys, got the same: %q", paid.EventKey)
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		if _, err := ParseWebhookEvent([]byte(`not json`)); err == nil {
			t.Error("expected an error for invalid JSON")
		}
	})
}

func generateTestKeyPair(t *testing.T) (privateKey *rsa.PrivateKey, publicKeyB64 string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	return priv, base64.StdEncoding.EncodeToString(pemBytes)
}

func sign(t *testing.T, priv *rsa.PrivateKey, body []byte) string {
	t.Helper()
	digest := sha256.Sum256(body)
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func TestVerifyWebhookSignature(t *testing.T) {
	priv, pubB64 := generateTestKeyPair(t)
	body := []byte(`{"event":"OPENPIX:CHARGE_COMPLETED","charge":{"correlationID":"c1"}}`)

	t.Run("valid signature passes", func(t *testing.T) {
		sig := sign(t, priv, body)
		if err := VerifyWebhookSignature(body, sig, pubB64); err != nil {
			t.Errorf("expected valid signature to pass, got: %v", err)
		}
	})

	t.Run("tampered body fails", func(t *testing.T) {
		sig := sign(t, priv, body)
		tampered := []byte(`{"event":"OPENPIX:CHARGE_COMPLETED","charge":{"correlationID":"c2"}}`)
		if err := VerifyWebhookSignature(tampered, sig, pubB64); err == nil {
			t.Error("expected tampered body to fail verification")
		}
	})

	t.Run("wrong key fails", func(t *testing.T) {
		sig := sign(t, priv, body)
		_, otherPubB64 := generateTestKeyPair(t)
		if err := VerifyWebhookSignature(body, sig, otherPubB64); err == nil {
			t.Error("expected wrong public key to fail verification")
		}
	})

	t.Run("missing signature fails", func(t *testing.T) {
		if err := VerifyWebhookSignature(body, "", pubB64); err == nil {
			t.Error("expected missing signature to fail")
		}
	})

	t.Run("malformed base64 signature fails", func(t *testing.T) {
		if err := VerifyWebhookSignature(body, "not-base64!!!", pubB64); err == nil {
			t.Error("expected malformed signature to fail")
		}
	})
}
