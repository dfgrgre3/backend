package authdto

// ─── Authentication Requests/Responses ─────────────────────────────

type LoginRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	RememberMe  bool   `json:"rememberMe"`
	DeviceName  string `json:"deviceName,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type LoginResponse struct {
	AccessToken  string  `json:"accessToken,omitempty"`
	RefreshToken string  `json:"refreshToken,omitempty"`
	User         UserDTO `json:"user,omitempty"`
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Username  string `json:"username,omitempty" binding:"omitempty,min=3,max=30"`
	Phone     string `json:"phone,omitempty"`
	Role      string `json:"role,omitempty" binding:"omitempty,oneof=STUDENT PARENT TEACHER"`
	Referral  string `json:"referralCode,omitempty"`
}

type RegisterResponse struct {
	Message string  `json:"message"`
	User    UserDTO `json:"user"`
}

type UserDTO struct {
	ID            string   `json:"id"`
	Email         string   `json:"email"`
	Name          string   `json:"name"`
	Username      string   `json:"username,omitempty"`
	Avatar        string   `json:"avatar,omitempty"`
	Role          string   `json:"role"`
	Status        string   `json:"status,omitempty"`
	EmailVerified bool     `json:"emailVerified"`
	PhoneVerified bool     `json:"phoneVerified,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyForgotPasswordCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

type VerifyForgotPasswordCodeResponse struct {
	ResetToken string `json:"resetToken,omitempty"`
	Message    string `json:"message"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

type VerifyEmailRequest struct {
	Code string `json:"code" binding:"required"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"omitempty,email"`
}

// ─── Update Profile ────────────────────────────────────────────────

type UpdateProfileRequest struct {
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Bio      string `json:"bio,omitempty"`
	Country  string `json:"country,omitempty"`
	Language string `json:"language,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type UpdateProfileResponse struct {
	Message string  `json:"message"`
	User    UserDTO `json:"user"`
}

// ─── Delete Account ────────────────────────────────────────────────

type DeleteAccountRequest struct {
	Password     string `json:"password" binding:"required"`
	Reason       string `json:"reason,omitempty"`
	Confirmation string `json:"confirmation" binding:"required,oneof=DELETE"`
}

type DeleteAccountResponse struct {
	Message string `json:"message"`
}

// ─── Sessions ──────────────────────────────────────────────────────

type SessionDTO struct {
	ID           string `json:"id"`
	DeviceType   string `json:"deviceType"`
	Browser      string `json:"browser"`
	OS           string `json:"os"`
	IP           string `json:"ip"`
	Country      string `json:"country,omitempty"`
	IsActive     bool   `json:"isActive"`
	RememberMe   bool   `json:"rememberMe"`
	LastAccessed string `json:"lastActive"`
	CreatedAt    string `json:"createdAt"`
	ExpiresAt    string `json:"expiresAt"`
	IsCurrent    bool   `json:"isCurrent,omitempty"`
}

type RevokeSessionRequest struct {
	SessionID string `json:"sessionId" binding:"required"`
}

type RevokeAllSessionsRequest struct {
	ExcludeCurrent bool `json:"excludeCurrent"`
}

// ─── 2FA / MFA ─────────────────────────────────────────────────────

type SetupMFARequest struct {
	Method string `json:"method" binding:"required,oneof=totp email sms"`
}

type SetupMFAResponse struct {
	Secret      string   `json:"secret,omitempty"`
	QRCodeURL   string   `json:"qrCodeUrl,omitempty"`
	BackupCodes []string `json:"backupCodes,omitempty"`
}

type VerifyMFARequest struct {
	ChallengeID string `json:"challengeId" binding:"required"`
	Code        string `json:"code" binding:"required"`
}

type EnableMFARequest struct {
	Code string `json:"code" binding:"required"`
}

type DisableMFARequest struct {
	Password string `json:"password" binding:"required"`
}

type GenerateBackupCodesResponse struct {
	BackupCodes []string `json:"backupCodes"`
}

// ─── Social Auth ───────────────────────────────────────────────────

type OAuthLoginRequest struct {
	Provider    string `json:"provider" binding:"required,oneof=google github microsoft facebook apple"`
	Token       string `json:"token" binding:"required"`
	RedirectURI string `json:"redirectUri"`
	DeviceName  string `json:"deviceName,omitempty"`
}

type OAuthCallbackRequest struct {
	Code     string `json:"code" binding:"required"`
	State    string `json:"state" binding:"required"`
	Provider string `json:"provider"`
}

type LinkProviderRequest struct {
	Provider string `json:"provider" binding:"required,oneof=google github microsoft facebook apple"`
	Code     string `json:"code" binding:"required"`
	State    string `json:"state" binding:"required"`
}

type UnlinkProviderRequest struct {
	Provider string `json:"provider" binding:"required,oneof=google github microsoft facebook apple"`
}

type LinkedAccountDTO struct {
	Provider string `json:"provider"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	LinkedAt string `json:"linkedAt"`
}

// ─── Token Validation ──────────────────────────────────────────────

type ValidateTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type ValidateTokenResponse struct {
	Valid       bool     `json:"valid"`
	UserID      string   `json:"userId,omitempty"`
	Role        string   `json:"role,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	ExpiresAt   string   `json:"expiresAt,omitempty"`
}

// ─── Account Recovery ──────────────────────────────────────────────

type AccountRecoveryRequest struct {
	Email       string `json:"email" binding:"required,email"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	Method      string `json:"method" binding:"required,oneof=email phone admin"`
}

type AccountRecoveryResponse struct {
	ChallengeID string `json:"challengeId,omitempty"`
	Message     string `json:"message"`
}

type RecoverAccountRequest struct {
	ChallengeID string `json:"challengeId" binding:"required"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}
