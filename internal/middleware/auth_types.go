package middleware

// ContextKey is a typed key for context values to avoid collisions.
type ContextKey string

const (
	UserContextKey  ContextKey = "user_id"
	RoleContextKey  ContextKey = "user_role"
	EmailContextKey ContextKey = "user_email"
)
