package userdomain

import (
	"context"
	"time"
)

// ListUsersResult is the paginated result returned by Repository.List.
type ListUsersResult struct {
	Users []*User
	Total int64
}

// Repository is the port for user persistence.
type Repository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListUsersFilter) (ListUsersResult, error)
	CountByRole(ctx context.Context, role Role) (int64, error)
	CountByStatus(ctx context.Context, status Status) (int64, error)
	CountCreatedSince(ctx context.Context, since time.Time) (int64, error)
}

// PasswordHasher is the port for password hashing.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
}

// EventPublisher is the port for publishing domain events.
type EventPublisher interface {
	Publish(ctx context.Context, event UserEvent) error
}

// UserEvent is emitted by the user domain.
type UserEvent struct {
	Type      string
	UserID    string
	Timestamp time.Time
	Data      map[string]interface{}
}
