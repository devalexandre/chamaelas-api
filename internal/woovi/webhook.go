package woovi

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"
)

// DefaultPublicKeyB64 is Woovi's published webhook-signing public key
// (base64-encoded PEM), from their signature validation docs.
const DefaultPublicKeyB64 = "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlHZk1BMEdDU3FHU0liM0RRRUJBUVVBQTRHTkFEQ0JpUUtCZ1FDLytOdElranpldnZxRCtJM01NdjNiTFhEdApwdnhCalk0QnNSclNkY2EzcnRBd01jUllZdnhTbmQ3amFnVkxwY3RNaU94UU84aWVVQ0tMU1dIcHNNQWpPL3paCldNS2Jxb0c4TU5waS91M2ZwNnp6MG1jSENPU3FZc1BVVUcxOWJ1VzhiaXM1WloySVpnQk9iV1NwVHZKMGNuajYKSEtCQUE4MkpsbitsR3dTMU13SURBUUFCCi0tLS0tRU5EIFBVQkxJQyBLRVktLS0tLQo="

const (
	eventChargeCompleted = "OPENPIX:CHARGE_COMPLETED"
	eventChargeExpired   = "OPENPIX:CHARGE_EXPIRED"
)

type WebhookEventType string

const (
	EventPaid    WebhookEventType = "paid"
	EventExpired WebhookEventType = "expired"
	EventIgnored WebhookEventType = "ignored"
)

type WebhookEvent struct {
	Type          WebhookEventType
	CorrelationID string
	// EventKey uniquely identifies this delivery for dedup — includes the
	// event type, not just the correlation ID, since the same charge
	// legitimately produces more than one event over its life (paid, then
	// later expired).
	EventKey   string
	ValueCents int
	PaidAt     *time.Time
}

type webhookPayload struct {
	Event  string `json:"event"`
	Evento string `json:"evento"` // Woovi's webhook-registration test ping: {"evento":"teste_webhook"}
	Charge struct {
		CorrelationID string `json:"correlationID"`
		Value         int    `json:"value"`
		PaidAt        string `json:"paidAt"`
	} `json:"charge"`
}

// ParseWebhookEvent parses a Woovi webhook body. Woovi's own
// webhook-registration test ping (no signature, `{"evento":"teste_webhook"}`)
// and any body missing an `event` key are both returned as EventIgnored —
// callers should respond 200 without erroring for those.
func ParseWebhookEvent(body []byte) (*WebhookEvent, error) {
	var wh webhookPayload
	if err := json.Unmarshal(body, &wh); err != nil {
		return nil, fmt.Errorf("payload inválido: %w", err)
	}

	if wh.Evento != "" || wh.Event == "" {
		return &WebhookEvent{Type: EventIgnored}, nil
	}

	evt := &WebhookEvent{
		CorrelationID: wh.Charge.CorrelationID,
		EventKey:      wh.Event + ":" + wh.Charge.CorrelationID,
		ValueCents:    wh.Charge.Value,
	}
	switch wh.Event {
	case eventChargeCompleted:
		evt.Type = EventPaid
		if t, err := time.Parse(time.RFC3339, wh.Charge.PaidAt); err == nil {
			evt.PaidAt = &t
		}
	case eventChargeExpired:
		evt.Type = EventExpired
	default:
		evt.Type = EventIgnored
	}
	return evt, nil
}

// VerifyWebhookSignature checks the RSA-SHA256 signature Woovi sends in the
// x-webhook-signature header. publicKeyB64 empty means use Woovi's own
// published default key.
func VerifyWebhookSignature(body []byte, signatureB64, publicKeyB64 string) error {
	if signatureB64 == "" {
		return fmt.Errorf("assinatura ausente")
	}
	if publicKeyB64 == "" {
		publicKeyB64 = DefaultPublicKeyB64
	}
	pub, err := parsePublicKey(publicKeyB64)
	if err != nil {
		return fmt.Errorf("chave pública do webhook inválida: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("assinatura não é base64: %w", err)
	}
	digest := sha256.Sum256(body)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		return fmt.Errorf("assinatura não confere")
	}
	return nil
}

func parsePublicKey(b64 string) (*rsa.PublicKey, error) {
	pemBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("PEM inválido")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("chave não é RSA")
	}
	return rsaPub, nil
}
