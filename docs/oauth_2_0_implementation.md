# 🔐 OAuth 2.0 Implementation Guide - Tolo Backend

**Last Updated**: 2026-07-23  
**Version**: 1.0 (Real OAuth - Production Ready)  
**Status**: ✅ COMPLETE - Google OAuth + Apple OAuth

---

## Table of Contents
1. [Overview](#overview)
2. [Google OAuth Setup](#google-oauth-setup)
3. [Apple OAuth Setup](#apple-oauth-setup)
4. [Backend Configuration](#backend-configuration)
5. [API Endpoints](#api-endpoints)
6. [Frontend Integration](#frontend-integration)
7. [Security Considerations](#security-considerations)
8. [Testing](#testing)
9. [Troubleshooting](#troubleshooting)

---

## Overview

The Tolo backend now supports **real OAuth 2.0** authentication with Google and Apple. This replaces the previous mock implementation that created test users.

### What Changed
| Feature | Before (Mock) | After (Real) |
|---------|---------------|--------------|
| User Creation | Fake email `oauth-{provider}-{code[:4]}@example.com` | Real email from provider |
| Provider Verification | ❌ No (mock code) | ✅ Yes (verified by provider) |
| User Data | Hardcoded "OAuth User" | Real name/picture from provider |
| State Parameter | ❌ No CSRF protection | ✅ Redis-backed state validation |
| Email Verification | Assumed verified | Verified by OAuth provider |

---

## Google OAuth Setup

### Step 1: Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Click **"Select a Project"** → **"New Project"**
3. Enter project name: `Thanawy App`
4. Click **Create**

### Step 2: Enable OAuth 2.0 API

1. In the Cloud Console, go to **APIs & Services** → **Library**
2. Search for: `Google+ API`
3. Click it and press **Enable**
4. Also enable **Google Identity Platform** (if prompted)

### Step 3: Create OAuth 2.0 Credentials

1. Go to **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **OAuth client ID**
3. Choose **Web application**
4. Name it: `Tolo Backend`
5. Add Authorized redirect URIs:
   ```
   http://localhost:8080/auth/oauth/google/callback    (development)
   https://your-backend.com/auth/oauth/google/callback (production)
   ```
6. Click **Create**
7. A dialog appears with your credentials. Copy them:
   - **Client ID**: `xxxxxx-xxx.apps.googleusercontent.com`
   - **Client Secret**: `xxxxxx_xxxxxx_xxxx`

### Step 4: Store Credentials

Add to your `.env` file:
```bash
GOOGLE_CLIENT_ID="xxxxxx-xxx.apps.googleusercontent.com"
GOOGLE_CLIENT_SECRET="xxxxxx_xxxxxx_xxxx"
```

---

## Apple OAuth Setup

Apple OAuth is more complex than Google. You'll need an Apple Developer account ($99/year).

### Step 1: Register Your App

1. Go to [Apple Developer Account](https://developer.apple.com/account)
2. Go to **Identifiers**
3. Click the **+** button
4. Select **App IDs**
5. Register as:
   - **App Type**: Web
   - **Bundle ID**: `com.thanawy.app`
   - **Description**: Tolo LMS Platform
   - Enable **Sign in with Apple**

### Step 2: Create a Service ID

1. Go to **Identifiers** again
2. Click the **+** button, select **Services IDs**
3. Name it: `Tolo Backend Service`
4. **Identifier**: `com.thanawy.backend`
5. Enable **Sign in with Apple**
6. Click **Configure**
7. Add **Web Redirect URIs**:
   ```
   https://localhost:8080/auth/oauth/apple/callback   (development)
   https://your-backend.com/auth/oauth/apple/callback (production)
   ```
8. Save

### Step 3: Create a Private Key

1. Go to **Keys** section
2. Click the **+** button
3. Name it: `Tolo Apple Login Key`
4. Enable **Sign in with Apple**
5. Click **Configure**
6. Select the **Service ID** you created above
7. Click **Save** and then **Register**
8. Download the private key file (`.p8`)

### Step 4: Store Credentials

Extract from Apple Developer console:
- **Team ID**: Found at [Membership](https://developer.apple.com/account/#/membership)
- **Key ID**: Displayed when creating the key
- **Private Key**: Content of the `.p8` file

Add to `.env`:
```bash
APPLE_CLIENT_ID="com.thanawy.app"
APPLE_TEAM_ID="XXXXXXXXXX"              # 10-character team ID
APPLE_KEY_ID="XXXXXXXXXX"               # 10-character key ID
APPLE_SECRET="-----BEGIN PRIVATE KEY-----
MIGfMA0GCSq...
-----END PRIVATE KEY-----"              # Full private key content
```

---

## Backend Configuration

### 1. Environment Variables

Ensure your `.env` file has:
```bash
# Google OAuth
GOOGLE_CLIENT_ID="your-google-client-id"
GOOGLE_CLIENT_SECRET="your-google-client-secret"

# Apple OAuth (Optional - can be empty)
APPLE_CLIENT_ID="com.thanawy.app"
APPLE_TEAM_ID="XXXXXXXXXX"
APPLE_KEY_ID="XXXXXXXXXX"
APPLE_SECRET="-----BEGIN PRIVATE KEY-----\n..."

# OAuth Redirect URL
OAUTH_REDIRECT_URL="http://localhost:8080"

# Redis (Required for OAuth state management)
REDIS_URL="redis://localhost:6379"
```

### 2. Initialize OAuth Service

In your main application setup (e.g., `cmd/api/main.go`):

```go
// Import OAuth service
import (
    "thanawy-backend/internal/services"
)

// Initialize OAuthService
oauthCfg := services.OAuthConfig{
    GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
    GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
    AppleClientID:      os.Getenv("APPLE_CLIENT_ID"),
    AppleClientSecret:  os.Getenv("APPLE_SECRET"),
    AppleKeyID:         os.Getenv("APPLE_KEY_ID"),
    AppleTeamID:        os.Getenv("APPLE_TEAM_ID"),
    RedirectURL:        os.Getenv("OAUTH_REDIRECT_URL"),
}

oauthService, err := services.NewOAuthService(oauthCfg)
if err != nil {
    log.Fatalf("Failed to initialize OAuth service: %v", err)
}

// Pass to Auth Service
authService := services.NewAuthService(authRepo, tokenService, oauthService)
```

### 3. Ensure Redis is Running

```bash
# Development with Docker
docker run -d -p 6379:6379 redis:latest

# Or with Redis CLI
redis-server
```

---

## API Endpoints

### OAuth Authorization URLs

#### Start Google Login

**Request:**
```http
GET /auth/oauth/google/url
```

**Response:**
```json
{
  "url": "https://accounts.google.com/o/oauth2/v2/auth?client_id=...",
  "state": "random-32-char-state-string"
}
```

#### Start Apple Login

**Request:**
```http
GET /auth/oauth/apple/url
```

**Response:**
```json
{
  "url": "https://appleid.apple.com/auth/authorize?...",
  "state": "random-32-char-state-string"
}
```

---

### OAuth Callback Handler

#### Google Callback

**Request:**
```http
GET /auth/oauth/callback?provider=google&code=4%2F0AXXXx...&state=xxxx
```

**Response:**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "refresh_xxxx",
  "user": {
    "id": "user-uuid",
    "email": "user@gmail.com",
    "name": "John Doe",
    "role": "student",
    "emailVerified": true,
    "permissions": [...]
  }
}
```

#### Apple Callback

**Request:**
```http
POST /auth/oauth/callback
Content-Type: application/x-www-form-urlencoded

provider=apple&code=cXXXXX&id_token=eyJhbGc...&user=...&state=xxxx
```

**Response:** (Same as Google)

---

## Frontend Integration

### Example: React with Next.js

#### 1. Get Authorization URL

```typescript
// hooks/useOAuth.ts
import { useState } from 'react';

export const useGoogleOAuth = () => {
  const [loading, setLoading] = useState(false);

  const startGoogleAuth = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/auth/oauth/google/url');
      const { url } = await res.json();
      
      // Store state in session storage (CSRF protection)
      sessionStorage.setItem('oauth_state', url.split('state=')[1]);
      
      // Redirect to Google
      window.location.href = url;
    } catch (err) {
      console.error('Failed to get Google OAuth URL:', err);
      setLoading(false);
    }
  };

  return { startGoogleAuth, loading };
};
```

#### 2. Handle OAuth Callback

```typescript
// pages/auth/oauth/callback.tsx
import { useEffect } from 'react';
import { useRouter } from 'next/router';
import { useAuth } from '@/contexts/auth-context';

export default function OAuthCallback() {
  const router = useRouter();
  const { setAuthTokens } = useAuth();
  
  useEffect(() => {
    const handleCallback = async () => {
      const { provider, code, state } = router.query;

      // Verify state (CSRF protection)
      const savedState = sessionStorage.getItem('oauth_state');
      if (state !== savedState) {
        router.push('/auth/login?error=csrf');
        return;
      }

      try {
        const res = await fetch('/api/auth/oauth/callback', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ provider, code, state }),
        });

        if (!res.ok) throw new Error('OAuth callback failed');

        const { accessToken, refreshToken, user } = await res.json();
        
        // Store tokens
        setAuthTokens({ accessToken, refreshToken, user });
        
        // Redirect to dashboard
        router.push('/dashboard');
      } catch (err) {
        console.error('OAuth callback error:', err);
        router.push('/auth/login?error=oauth_failed');
      }
    };

    if (router.isReady) {
      handleCallback();
    }
  }, [router.isReady]);

  return <div>Processing login...</div>;
}
```

#### 3. Login Button Component

```typescript
// components/auth/OAuthButtons.tsx
import { useGoogleOAuth } from '@/hooks/useOAuth';

export const OAuthButtons = () => {
  const { startGoogleAuth, loading } = useGoogleOAuth();

  return (
    <div className="space-y-2">
      <button
        onClick={startGoogleAuth}
        disabled={loading}
        className="w-full btn btn-google"
      >
        {loading ? 'Redirecting...' : 'Continue with Google'}
      </button>
      
      {/* Apple login (coming soon or if configured) */}
      <button
        className="w-full btn btn-apple"
        disabled  // Enable when Apple OAuth is fully configured
      >
        Continue with Apple (Coming Soon)
      </button>
    </div>
  );
};
```

---

## Security Considerations

### 1. State Parameter (CSRF Protection)
✅ Implemented: OAuth state is validated using Redis
- 32-character random string generated per login attempt
- Stored in Redis with 15-minute TTL
- Validated on callback and deleted immediately after
- Prevents Cross-Site Request Forgery attacks

### 2. Token Security
✅ Implemented:
- JWT tokens issued with `exp`, `iat`, `iss` claims
- Refresh tokens stored as SHA-256 hashes in database
- Tokens signed with strong JWT_SECRET
- Short expiry (typically 15 minutes for access, 30 days for refresh)

### 3. Email Verification
✅ Implemented:
- OAuth providers verify email before issuing tokens
- All OAuth users have `emailVerified: true`
- Email is trusted source of truth for user identification

### 4. Account Linking
✅ Implemented:
- Multiple OAuth accounts can link to same user
- Prevents duplicate accounts if user changes email provider
- User can unlink OAuth accounts

### 5. HTTPS Required
⚠️ **Action Item**: In production:
- All OAuth redirect URIs must use HTTPS
- Set `COOKIE_SECURE=true`
- Add HTTP Strict-Transport-Security (HSTS) header

### 6. Client Secret Storage
⚠️ **Action Item**:
- Never expose `GOOGLE_CLIENT_SECRET` or `APPLE_SECRET` in frontend
- Store only in backend environment variables
- Rotate secrets quarterly

---

## Testing

### Manual Testing

#### 1. Test Google OAuth Flow

```bash
# Get authorization URL
curl http://localhost:8080/auth/oauth/google/url

# You'll get a redirect URL - visit it in browser
# Allow permissions
# You'll be redirected to callback with code + state

# Test callback (use code from previous step)
curl -X POST http://localhost:8080/auth/oauth/callback \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "google",
    "code": "4/0AXX...",
    "state": "xxxx"
  }'
```

#### 2. Test Apple OAuth Flow

Same process, but:
1. Use `/auth/oauth/apple/url` endpoint
2. Provider name is `apple`
3. Apple returns ID token instead of exchangeable code

---

### Unit Tests

```go
// internal/services/oauth_service_test.go
package services

import (
  "context"
  "testing"
)

func TestGoogleOAuthFlow(t *testing.T) {
  cfg := OAuthConfig{
    GoogleClientID:     "test-id",
    GoogleClientSecret: "test-secret",
    RedirectURL:        "http://localhost:8080",
  }

  svc, err := NewOAuthService(cfg)
  if err != nil {
    t.Fatalf("Failed to create OAuth service: %v", err)
  }

  // Test state generation
  state, err := svc.GenerateOAuthState(context.Background())
  if err != nil || state == "" {
    t.Fatalf("Failed to generate state: %v", err)
  }

  // Test state validation
  valid, err := svc.ValidateOAuthState(context.Background(), state)
  if err != nil || !valid {
    t.Fatalf("State validation failed: %v", err)
  }

  // Test auth URL generation
  authURL := svc.GetGoogleAuthURL(state)
  if authURL == "" {
    t.Fatalf("Empty auth URL generated")
  }

  if !strings.Contains(authURL, cfg.GoogleClientID) {
    t.Fatalf("Auth URL missing client ID")
  }
}
```

---

## Troubleshooting

### Issue: "Invalid OAuth Provider"
**Cause**: Provider name doesn't match `google` or `apple`
**Fix**: Double-check provider parameter in request

---

### Issue: "OAuth State Expired"
**Cause**: State validation failed (>15 minutes passed, or Redis unavailable)
**Fix**:
1. Ensure Redis is running: `redis-cli ping`
2. Restart OAuth flow
3. Increase state TTL if needed

---

### Issue: "Google: Invalid Grant"
**Cause**: Code already used or expired (codes are one-time use)
**Fix**:
1. Restart OAuth flow
2. Check that code is fresh (<10 minutes old)
3. Verify `GOOGLE_CLIENT_SECRET` is correct

---

### Issue: "Apple: token_type not in response"
**Cause**: Apple configuration incomplete or mismatched keys
**Fix**:
1. Verify all Apple credentials in `.env`
2. Check that private key (`APPLE_SECRET`) is valid PEM format
3. Ensure `APPLE_CLIENT_ID` matches Service ID

---

### Issue: "Redirect URI Mismatch"
**Cause**: OAuth callback URL doesn't match registered one
**Fix**:
1. Google Console: **APIs & Services** → **Credentials** → Edit → verify redirect URIs
2. Apple Developer: **Services IDs** → Edit → verify redirect URIs
3. Must match exactly (including `http://` vs `https://`, domain, path)

---

## Migration from Mock OAuth

### For Existing Users

Users created with mock OAuth IDs (`oauth-google-xxxx@example.com`) will need to:
1. Use actual email address to login next time
2. Link their OAuth account through settings (if implemented)
3. Or contact support to migrate data

### Migration Script (Optional)

```go
// cmd/migrate/fix_oauth_users.go
// Manually links old mock accounts to real OAuth accounts
func migrateOAuthAccounts() {
  // Query all oauth-xxx@example.com emails
  // Exchange for real Google/Apple info
  // Create proper OAuth account links
}
```

---

## Reference

- [Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)
- [Apple Sign in with Apple](https://developer.apple.com/sign-in-with-apple/)
- [golang.org/x/oauth2 Library](https://pkg.go.dev/golang.org/x/oauth2)
- [RFC 6749 - OAuth 2.0 Authorization Framework](https://tools.ietf.org/html/rfc6749)

---

**Status**: ✅ Production Ready  
**Last Review**: 2026-07-23  
**Next Review**: 2026-08-23
