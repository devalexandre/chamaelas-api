// Package woovi is a minimal client for the Woovi (OpenPix) PIX API pieces
// the driver/passenger wallet needs: creating a subaccount for a Pix key,
// checking its balance, moving money between subaccounts, withdrawing a
// subaccount's balance out as a real Pix transfer, and creating a plain
// (no-split) charge for a credit top-up.
//
// Docs: https://developers.woovi.com. Mirrors internal/pagarme/client.go's
// style — plain functions taking credentials as params, stdlib net/http —
// rather than a stateful client struct.
package woovi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	SandboxBaseURL    = "https://api.woovi-sandbox.com"
	ProductionBaseURL = "https://api.woovi.com"
)

// BaseURL maps a woovi_settings.environment value to the actual API host.
func BaseURL(environment string) string {
	if environment == "production" {
		return ProductionBaseURL
	}
	return SandboxBaseURL
}

func doJSON(appID, baseURL, method, path string, reqBody any) (int, []byte, error) {
	var reader io.Reader
	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, strings.TrimRight(baseURL, "/")+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", appID)

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("falha ao conectar com a Woovi: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return res.StatusCode, body, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// EnsureRecipient creates (idempotently) the subaccount for pixKey and
// returns Woovi's CANONICAL form of that key (CPF/CNPJ digits-only, phone
// with "+55") — that's the form every later call (balance/transfer/
// withdraw) must use, since querying by the raw key returns "not found".
func EnsureRecipient(appID, baseURL, pixKey, name string) (string, error) {
	if appID == "" {
		return "", fmt.Errorf("gateway de pagamento não configurado")
	}
	status, body, err := doJSON(appID, baseURL, http.MethodPost, "/api/v1/subaccount",
		map[string]string{"pixKey": pixKey, "name": name})
	if err != nil {
		return "", err
	}
	ok := status >= 200 && status <= 299
	if !ok && !strings.Contains(strings.ToLower(string(body)), "already") {
		return "", fmt.Errorf("woovi: subaccount status %d: %s", status, truncate(body, 300))
	}

	canonical := pixKey
	var out struct {
		SubAccount struct {
			PixKey string `json:"pixKey"`
		} `json:"subAccount"`
	}
	if err := json.Unmarshal(body, &out); err == nil && out.SubAccount.PixKey != "" {
		canonical = out.SubAccount.PixKey
	}
	return canonical, nil
}

// RecipientBalanceCents returns the subaccount balance (in cents) for the
// already-canonical pixKey.
func RecipientBalanceCents(appID, baseURL, pixKey string) (int, error) {
	if appID == "" {
		return 0, fmt.Errorf("gateway de pagamento não configurado")
	}
	status, body, err := doJSON(appID, baseURL, http.MethodGet, "/api/v1/subaccount/"+url.PathEscape(pixKey), nil)
	if err != nil {
		return 0, err
	}
	if status < 200 || status > 299 {
		return 0, fmt.Errorf("woovi: subaccount balance status %d: %s", status, truncate(body, 300))
	}
	var out struct {
		SubAccount struct {
			Balance int `json:"balance"`
		} `json:"subAccount"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("woovi: resposta inválida: %w", err)
	}
	return out.SubAccount.Balance, nil
}

var nonDigit = regexp.MustCompile(`\D`)

// OnlyDigits strips everything but digits — used to normalize a CPF before
// using it as a default Pix key for auto-provisioning a subaccount.
func OnlyDigits(s string) string {
	return nonDigit.ReplaceAllString(s, "")
}

// pixKeyType infers a Pix key's type from its format, required by the
// subaccount-transfer endpoint.
func pixKeyType(key string) string {
	digits := nonDigit.ReplaceAllString(key, "")
	switch {
	case strings.Contains(key, "@"):
		return "EMAIL"
	case strings.HasPrefix(key, "+"):
		return "PHONE"
	case len(digits) == 11 && len(key) == 11:
		return "CPF"
	case len(digits) == 14 && len(key) == 14:
		return "CNPJ"
	default:
		return "RANDOM"
	}
}

// RecipientTransfer moves balance between two subaccounts — an internal
// ledger move, not a real Pix transfer — used both to credit a driver's
// wallet with her ride earnings and to collect the withdrawal fee.
func RecipientTransfer(appID, baseURL, fromPixKey, toPixKey string, cents int) error {
	if appID == "" {
		return fmt.Errorf("gateway de pagamento não configurado")
	}
	status, body, err := doJSON(appID, baseURL, http.MethodPost, "/api/v1/subaccount/transfer", map[string]any{
		"value":          cents,
		"fromPixKey":     fromPixKey,
		"fromPixKeyType": pixKeyType(fromPixKey),
		"toPixKey":       toPixKey,
		"toPixKeyType":   pixKeyType(toPixKey),
	})
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("woovi: subaccount transfer status %d: %s", status, truncate(body, 300))
	}
	return nil
}

// RecipientWithdraw sweeps the ENTIRE subaccount balance out as a real Pix
// transfer to that subaccount's own registered Pix key — there is no
// partial-amount parameter.
func RecipientWithdraw(appID, baseURL, pixKey string) (transactionID string, cents int, err error) {
	if appID == "" {
		return "", 0, fmt.Errorf("gateway de pagamento não configurado")
	}
	status, body, err := doJSON(appID, baseURL, http.MethodPost, "/api/v1/subaccount/"+url.PathEscape(pixKey)+"/withdraw", nil)
	if err != nil {
		return "", 0, err
	}
	if status < 200 || status > 299 {
		return "", 0, fmt.Errorf("woovi: subaccount withdraw status %d: %s", status, truncate(body, 300))
	}
	var out struct {
		Transaction struct {
			Status        string `json:"status"`
			Value         int    `json:"value"`
			CorrelationID string `json:"correlationID"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, fmt.Errorf("woovi: resposta inválida: %w", err)
	}
	return out.Transaction.CorrelationID, out.Transaction.Value, nil
}

// ChargeInput is what's needed to create a plain (no-split) Pix charge —
// used for credit top-ups, where 100% of the value stays with the platform.
type ChargeInput struct {
	CorrelationID string
	Cents         int
	Comment       string
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
}

type ChargeResult struct {
	BRCode      string
	QRCodeImage string
	PaymentLink string
	Status      string
	ExpiresAt   *time.Time
}

type chargeRequestCustomer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type chargeRequest struct {
	CorrelationID string                 `json:"correlationID"`
	Value         int                    `json:"value"`
	Comment       string                 `json:"comment,omitempty"`
	Customer      *chargeRequestCustomer `json:"customer,omitempty"`
}

type chargeResponse struct {
	Charge struct {
		Status         string `json:"status"`
		BRCode         string `json:"brCode"`
		QRCodeImage    string `json:"qrCodeImage"`
		PaymentLinkURL string `json:"paymentLinkUrl"`
		ExpiresDate    string `json:"expiresDate"`
	} `json:"charge"`
	BRCode string `json:"brCode"`
	Error  string `json:"error"`
}

// CreateCharge creates a Pix charge with no split — the full value stays in
// the platform's own Woovi account, which is exactly what a credit top-up
// needs (the driver/passenger is paying the platform, not another party).
func CreateCharge(appID, baseURL string, in ChargeInput) (*ChargeResult, error) {
	if appID == "" {
		return nil, fmt.Errorf("gateway de pagamento não configurado")
	}
	reqBody := chargeRequest{
		CorrelationID: in.CorrelationID,
		Value:         in.Cents,
		Comment:       in.Comment,
	}
	if in.CustomerName != "" || in.CustomerEmail != "" || in.CustomerPhone != "" {
		reqBody.Customer = &chargeRequestCustomer{Name: in.CustomerName, Email: in.CustomerEmail, Phone: in.CustomerPhone}
	}

	status, body, err := doJSON(appID, baseURL, http.MethodPost, "/api/v1/charge", reqBody)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, fmt.Errorf("woovi: status %d: %s", status, truncate(body, 300))
	}

	var out chargeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("woovi: resposta inválida: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("woovi: %s", out.Error)
	}

	brCode := out.Charge.BRCode
	if brCode == "" {
		brCode = out.BRCode
	}
	result := &ChargeResult{
		BRCode:      brCode,
		QRCodeImage: out.Charge.QRCodeImage,
		PaymentLink: out.Charge.PaymentLinkURL,
		Status:      out.Charge.Status,
	}
	if out.Charge.ExpiresDate != "" {
		if t, err := time.Parse(time.RFC3339, out.Charge.ExpiresDate); err == nil {
			result.ExpiresAt = &t
		}
	}
	return result, nil
}

// WithdrawFeeCents is the platform's withdrawal fee: R$1.00 for a balance
// under R$500.00, free at or above it.
func WithdrawFeeCents(balanceCents int) int {
	const (
		feeCents       = 100
		feeExemptCents = 50000
	)
	if balanceCents <= 0 || balanceCents >= feeExemptCents {
		return 0
	}
	return feeCents
}
