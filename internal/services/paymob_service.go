package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type PaymobService struct {
	APIKey              string
	HMACSecret          string
	CardIntegrationID   string
	WalletIntegrationID string
	FawryIntegrationID  string
	IframeID            string
	BaseURL             string
}

func NewPaymobService() *PaymobService {
	return &PaymobService{
		APIKey:              os.Getenv("PAYMOB_API_KEY"),
		HMACSecret:          os.Getenv("PAYMOB_HMAC_SECRET"),
		CardIntegrationID:   os.Getenv("PAYMOB_CARD_INTEGRATION_ID"),
		WalletIntegrationID: os.Getenv("PAYMOB_WALLET_INTEGRATION_ID"),
		FawryIntegrationID:  os.Getenv("PAYMOB_FAWRY_INTEGRATION_ID"),
		IframeID:            os.Getenv("PAYMOB_IFRAME_ID"),
		BaseURL:             "https://accept.paymob.com/api",
	}
}

// 1. Authentication with circuit breaker protection
func (s *PaymobService) Authenticate() (string, error) {
	service := GetCircuitBreakerService()

	var result string
	err := service.CallExternalAPI("paymob-api", func() error {
		payload := map[string]string{"api_key": s.APIKey}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", s.BaseURL+"/auth/tokens", bytes.NewBuffer(body))
		req.Header.Set(headerContentType, contentTypeJSON)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("paymob authentication failed: %d", resp.StatusCode)
		}

		var resultStruct struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&resultStruct); err != nil {
			return err
		}

		result = resultStruct.Token
		return nil
	})

	return result, err
}

// 2. Order Registration
func (s *PaymobService) RegisterOrder(authToken string, amountCents int64, items []interface{}) (int64, error) {
	payload := map[string]interface{}{
		"auth_token":      authToken,
		"delivery_needed": "false",
		"amount_cents":    amountCents,
		"currency":        "EGP",
		"items":           items,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", s.BaseURL+"/ecommerce/orders", bytes.NewBuffer(body))
	req.Header.Set(headerContentType, contentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.ID, nil
}

// 3. Payment Key Generation
func (s *PaymobService) GetPaymentKey(authToken string, orderID, amountCents int64, integrationID string, billingData map[string]string) (string, error) {
	// Ensure billing data has required fields
	required := []string{"first_name", "last_name", "email", "phone_number"}
	for _, f := range required {
		if billingData[f] == "" {
			billingData[f] = "N/A"
		}
	}
	// Defaults for optional fields using neutral placeholder (N/A)
	optionalFields := []string{"apartment", "floor", "street", "building", "shipping_method", "postal_code", "city", "country", "state"}
	for _, f := range optionalFields {
		if billingData[f] == "" {
			billingData[f] = "N/A"
		}
	}

	payload := map[string]interface{}{
		"auth_token":           authToken,
		"amount_cents":         amountCents,
		"expiration":           3600,
		"order_id":             orderID,
		"billing_data":         billingData,
		"currency":             "EGP",
		"integration_id":       integrationID,
		"lock_order_when_paid": "false",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", s.BaseURL+"/acceptance/payment_keys", bytes.NewBuffer(body))
	req.Header.Set(headerContentType, contentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}

// Wallet Payment Request (for Vodafone Cash, etc.)
func (s *PaymobService) CreateWalletRequest(paymentKey, phoneNumber string) (string, error) {
	payload := map[string]interface{}{
		"source": map[string]string{
			"identifier": phoneNumber,
			"subtype":    "WALLET",
		},
		"payment_token": paymentKey,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", s.BaseURL+"/acceptance/payments/pay", bytes.NewBuffer(body))
	req.Header.Set(headerContentType, contentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		IframeRedirectionURL string `json:"iframe_redirection_url"`
		RedirectURL          string `json:"redirect_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.RedirectURL != "" {
		return result.RedirectURL, nil
	}
	return result.IframeRedirectionURL, nil
}

// VerifyHMAC verifies the HMAC hash from Paymob webhook
// Paymob sends "hmac" field in the webhook payload that needs to be verified
// The HMAC is computed from specific fields in the payload using SHA512
func (s *PaymobService) VerifyHMAC(payload map[string]interface{}) bool {
	if s.HMACSecret == "" {
		// CRITICAL SECURITY FIX: Never skip HMAC verification in production
		// If HMAC secret is not configured, reject the webhook
		return false
	}

	// Extract HMAC from payload
	hmacFromPayload, ok := payload["hmac"].(string)
	if !ok || hmacFromPayload == "" {
		return false
	}

	// Paymob puts transaction data inside the "obj" field of the webhook payload.
	// If "obj" is present, extract it. Otherwise fallback to root payload.
	obj, ok := payload["obj"].(map[string]interface{})
	if !ok {
		obj = payload
	}

	// Build the string to hash according to Paymob's documentation
	// The fields are concatenated with empty strings for missing values
	fields := []string{
		"amount_cents",
		"created_at",
		"currency",
		"error_occured",
		"has_parent_transaction",
		"id",
		"integration_id",
		"is_3d_secure",
		"is_auth",
		"is_capture",
		"is_refunded",
		"is_standalone_payment",
		"is_voided",
		"order",
		"owner",
		"pending",
		"source_data.pan",
		"source_data.sub_type",
		"source_data.type",
		"success",
	}

	var builder strings.Builder
	for _, field := range fields {
		val := getNestedValue(obj, field)
		builder.WriteString(val)
	}

	// Compute HMAC
	h := hmac.New(sha512.New, []byte(s.HMACSecret))
	h.Write([]byte(builder.String()))
	expectedHMAC := hex.EncodeToString(h.Sum(nil))

	// Compare (use constant time comparison to prevent timing attacks)
	return hmac.Equal([]byte(expectedHMAC), []byte(hmacFromPayload))
}

// getNestedValue extracts a value from a nested map using dot notation.
// Converts values to strings, handling float64 serialization carefully (e.g. avoiding scientific notation).
func getNestedValue(data map[string]interface{}, key string) string {
	keys := strings.Split(key, ".")
	current := data
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key
			val, ok := current[k]
			if !ok || val == nil {
				return ""
			}
			
			switch v := val.(type) {
			case float64:
				// JSON numbers are parsed as float64. Format integers correctly to avoid scientific notation
				// or trailing decimal points (e.g. 1.23456e+09 or 123456.0).
				if v == float64(int64(v)) {
					return fmt.Sprintf("%d", int64(v))
				}
				return fmt.Sprintf("%f", v)
			case float32:
				if v == float32(int32(v)) {
					return fmt.Sprintf("%d", int32(v))
				}
				return fmt.Sprintf("%f", v)
			case int:
				return fmt.Sprintf("%d", v)
			case int64:
				return fmt.Sprintf("%d", v)
			case bool:
				return fmt.Sprintf("%v", v)
			case string:
				return v
			default:
				return fmt.Sprintf("%v", v)
			}
		}
		// Navigate deeper
		next, ok := current[k].(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}
