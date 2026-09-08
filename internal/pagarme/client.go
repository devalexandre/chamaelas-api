// Package pagarme is a minimal client for the pieces of the Pagar.me v5 API
// the admin panel needs directly — right now just checking the platform's
// own recipient balance. Ride charges and driver payouts (split via
// Recebedores) are the next phase, once real sandbox keys are in place.
package pagarme

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.pagar.me/core/v5"

type Balance struct {
	Currency           string `json:"currency"`
	AvailableAmount    int64  `json:"available_amount"`
	WaitingFundsAmount int64  `json:"waiting_funds_amount"`
	TransferredAmount  int64  `json:"transferred_amount"`
}

// GetRecipientBalance calls GET /recipients/{id}/balance using HTTP Basic
// auth with the secret key as username (Pagar.me's convention — same
// endpoint and key work for both sandbox and production environments).
func GetRecipientBalance(secretKey, recipientID string) (*Balance, error) {
	if secretKey == "" || recipientID == "" {
		return nil, fmt.Errorf("secret key e recipient ID são obrigatórios")
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/recipients/"+recipientID+"/balance", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(secretKey, "")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar com a Pagar.me: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("chave secreta inválida")
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("recipient ID não encontrado")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pagar.me retornou status %d", res.StatusCode)
	}

	var balance Balance
	if err := json.NewDecoder(res.Body).Decode(&balance); err != nil {
		return nil, fmt.Errorf("resposta inesperada da pagar.me: %w", err)
	}
	return &balance, nil
}

// RecipientInput carries what's needed to create a driver's Recebedor:
// identity (for register_information) and the bank account payouts land
// on — Pagar.me's v5 Recipients API requires a full bank account, a bare
// Pix key alone isn't accepted by the documented endpoint.
type RecipientInput struct {
	Name              string
	Email             string
	Document          string // CPF, digits only
	BankCode          string
	BankBranch        string
	BankAccountNumber string
	BankAccountType   string // "checking" | "savings"
}

type Recipient struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// CreateRecipient calls POST /recipients to onboard a driver as a
// marketplace sub-account, so future ride payments can be split to her
// directly and she can be paid out from her own balance.
func CreateRecipient(secretKey string, input RecipientInput) (*Recipient, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("gateway de pagamento não configurado (secret key ausente)")
	}
	if input.BankCode == "" || input.BankAccountNumber == "" {
		return nil, fmt.Errorf("dados bancários da motorista incompletos")
	}

	body := map[string]any{
		"register_information": map[string]any{
			"name":     input.Name,
			"email":    input.Email,
			"document": input.Document,
			"type":     "individual",
		},
		"transfer_settings": map[string]any{
			"transfer_enabled":  true,
			"transfer_interval": "Daily",
		},
		"default_bank_account": map[string]any{
			"holder_name":     input.Name,
			"holder_type":     "individual",
			"holder_document": input.Document,
			"bank":            input.BankCode,
			"branch_number":   input.BankBranch,
			"account_number":  input.BankAccountNumber,
			"type":            input.BankAccountType,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/recipients", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(secretKey, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar com a Pagar.me: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("pagar.me retornou status %d: %s", res.StatusCode, string(respBody))
	}

	var recipient Recipient
	if err := json.NewDecoder(res.Body).Decode(&recipient); err != nil {
		return nil, fmt.Errorf("resposta inesperada da pagar.me: %w", err)
	}
	return &recipient, nil
}
