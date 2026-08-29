package protected

import (
	"net/http"
	authdto "thanawy-backend/internal/application/dto"
	"thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// Password and account-recovery endpoints: forgot/verify/reset password,
// email verification, change-password, and the account-recovery ticket flow.
// Split out of auth_handler.go for readability — same package, same
// *AuthHandler receiver, no behavior change.

// ─────────────────────────────────────────────
//  Forgot Password
// ─────────────────────────────────────────────

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req authdto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		response.ErrorDetail(c, http.StatusInternalServerError, "Failed to process password reset request", err)
		return
	}

	response.Success(c, gin.H{
		"message": "If an account with this email exists, a verification code has been sent.",
	})
}

// ─────────────────────────────────────────────
//  Verify Forgot Password Code
// ─────────────────────────────────────────────

func (h *AuthHandler) VerifyForgotPasswordCode(c *gin.Context) {
	var req authdto.VerifyForgotPasswordCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	resetToken, err := h.authService.VerifyForgotPasswordCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, authdto.VerifyForgotPasswordCodeResponse{
		ResetToken: resetToken,
		Message:    "Code verified successfully. You can now reset your password.",
	})
}

// ─────────────────────────────────────────────
//  Reset Password
// ─────────────────────────────────────────────

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req authdto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Password reset successfully. Please login with your new password."})
}

// ─────────────────────────────────────────────
//  Verify Email
// ─────────────────────────────────────────────

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	userIDValue, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := userIDValue.(string)

	var req authdto.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.VerifyEmail(c.Request.Context(), userID, req.Code); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Email verified successfully"})
}

// ─────────────────────────────────────────────
//  Resend Verification Email
// ─────────────────────────────────────────────

func (h *AuthHandler) ResendVerification(c *gin.Context) {
	userIDValue, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := userIDValue.(string)

	var req authdto.ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// No body, use user ID
	}

	if err := h.authService.ResendVerificationEmail(c.Request.Context(), userID); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Verification code sent successfully"})
}

// ─────────────────────────────────────────────
//  Change Password
// ─────────────────────────────────────────────

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req authdto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID.(string), req.OldPassword, req.NewPassword); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Clear cookies to force re-login
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{"message": "Password changed successfully. Please login again."})
}

// ─────────────────────────────────────────────
//  Account Recovery
// ─────────────────────────────────────────────

func (h *AuthHandler) AccountRecovery(c *gin.Context) {
	var req authdto.AccountRecoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	ticket, err := h.authService.InitiateAccountRecovery(c.Request.Context(), req.Email, req.Method)
	if err != nil {
		response.ErrorDetail(c, http.StatusInternalServerError, "Failed to initiate account recovery", err)
		return
	}

	response.Success(c, authdto.AccountRecoveryResponse{
		Ticket:  ticket,
		Message: "Recovery initiated. Check your email/phone for verification code.",
	})
}

// ─────────────────────────────────────────────
//  Recover Account (Finalize)
// ─────────────────────────────────────────────

func (h *AuthHandler) RecoverAccount(c *gin.Context) {
	var req authdto.RecoverAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.FinalizeAccountRecovery(c.Request.Context(), req.Ticket, req.Code, req.NewPassword); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Account recovered successfully. Please login with your new password."})
}
