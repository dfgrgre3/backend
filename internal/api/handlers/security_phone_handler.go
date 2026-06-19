package handlers

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
)

func isTwilioConfigured() (string, string, string, bool) {
	accountSID := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	fromNumber := os.Getenv("TWILIO_FROM_NUMBER")
	if fromNumber == "" {
		fromNumber = os.Getenv("TWILIO_PHONE_NUMBER")
	}

	accountSID = strings.TrimSpace(accountSID)
	authToken = strings.TrimSpace(authToken)
	fromNumber = strings.TrimSpace(fromNumber)

	if accountSID == "" || strings.Contains(accountSID, "CHANGE_ME") || strings.Contains(accountSID, "YOUR_") {
		return "", "", "", false
	}
	if authToken == "" || strings.Contains(authToken, "CHANGE_ME") || strings.Contains(authToken, "YOUR_") {
		return "", "", "", false
	}
	if fromNumber == "" || strings.Contains(fromNumber, "CHANGE_ME") || strings.Contains(fromNumber, "YOUR_") {
		return "", "", "", false
	}

	return accountSID, authToken, fromNumber, true
}

func sendTwilioSMS(accountSID, authToken, fromNumber, toNumber, otpCode string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID)

	data := url.Values{}
	data.Set("To", toNumber)
	data.Set("From", fromNumber)
	data.Set("Body", fmt.Sprintf("Your Thanawy verification code is: %s", otpCode))

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(accountSID, authToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("twilio api returned status %d: %s", resp.StatusCode, buf.String())
	}

	return nil
}

// SendPhoneVerification handles POST /api/auth/verify-phone/send
func SendPhoneVerification(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number is required"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Clean/validate phone number (simple validation)
	if len(req.Phone) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid phone number format"})
		return
	}

	// Generate 6-digit OTP
	const digits = "0123456789"
	otpBytes := make([]byte, 6)
	if _, err := rand.Read(otpBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification code"})
		return
	}
	for i, b := range otpBytes {
		otpBytes[i] = digits[b%10]
	}
	otpCode := string(otpBytes)

	// Save to user model
	expiresAt := time.Now().Add(10 * time.Minute)
	now := time.Now()

	err := db.DB.Model(&models.User{}).Where(idQuery, userID).Updates(map[string]interface{}{
		"phone":                        req.Phone,
		"phone_verification_otp":      otpCode,
		"phone_verification_expires":  expiresAt,
		"phone_verification_attempts": 0,
		"phone_verification_last_sent": now,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update phone settings"})
		return
	}

	// Check if Twilio is configured
	accountSID, authToken, fromNumber, isConfigured := isTwilioConfigured()
	if isConfigured {
		if err := sendTwilioSMS(accountSID, authToken, fromNumber, req.Phone, otpCode); err != nil {
			log.Printf("[Twilio Error] Failed to send SMS to %s: %v", req.Phone, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification SMS via Twilio"})
			return
		}
		log.Printf("[Twilio SMS] Verification code sent to %s", req.Phone)
	} else {
		// Mock SMS sending by logging to console
		log.Printf("[SMS MOCK] Verification code for user %v (%s): %s", userID, req.Phone, otpCode)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Verification code sent successfully"})
}

// VerifyPhoneVerification handles POST /api/auth/verify-phone/verify
func VerifyPhoneVerification(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code is required"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.PhoneVerificationOTP == nil || *user.PhoneVerificationOTP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No pending verification found"})
		return
	}

	if user.PhoneVerificationExpires == nil || user.PhoneVerificationExpires.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code has expired"})
		return
	}

	if user.PhoneVerificationAttempts >= 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many failed attempts. Please request a new code."})
		return
	}

	if *user.PhoneVerificationOTP != req.Code {
		// Increment attempts
		db.DB.Model(&user).Update("phone_verification_attempts", user.PhoneVerificationAttempts+1)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}

	// Success
	err := db.DB.Model(&user).Updates(map[string]interface{}{
		"phone_verified":              true,
		"phone_verification_otp":      nil,
		"phone_verification_expires":  nil,
		"phone_verification_attempts": 0,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete phone verification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Phone verified successfully"})
}
