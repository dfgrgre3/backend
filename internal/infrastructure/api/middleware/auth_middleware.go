package middleware

// Auth middleware has been split across dedicated files for clarity:
// - auth_core.go for authentication and caching
// - auth_guards.go for role/permission guards
// - auth_impersonation.go for impersonation helpers
// - auth_cors.go for CORS handling
//
// This file remains as a lightweight entry point for the package.
