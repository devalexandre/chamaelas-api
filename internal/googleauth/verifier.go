// Package googleauth verifies Google Sign-In ID tokens offline (signature +
// audience check against Google's published keys, no network round-trip per
// verification beyond the one-time key fetch google.golang.org/api/idtoken
// itself caches). Mirrors the pattern already proven in the sibling
// agenda_api project (internal/infra/google/verifier.go).
package googleauth

import (
	"context"
	"errors"

	"google.golang.org/api/idtoken"
)

// Claims is the subset of a verified Google ID token this app needs.
type Claims struct {
	Sub           string // stable, unique Google account id
	Email         string
	EmailVerified bool
	Name          string
}

// Verify validates idToken against audience (the OAuth web client's ID) and
// extracts the claims chamaelas-api needs to resolve or create an account.
func Verify(ctx context.Context, idToken, audience string) (Claims, error) {
	payload, err := idtoken.Validate(ctx, idToken, audience)
	if err != nil {
		return Claims{}, err
	}
	sub := payload.Subject
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	if sub == "" || email == "" {
		return Claims{}, errors.New("googleauth: token missing sub/email")
	}
	return Claims{Sub: sub, Email: email, EmailVerified: emailVerified, Name: name}, nil
}
