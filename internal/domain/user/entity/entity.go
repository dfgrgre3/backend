package userdomain

import "time"

type Role string

const (
	RoleAdmin      Role = "ADMIN"
	RoleTeacher    Role = "TEACHER"
	RoleStudent    Role = "STUDENT"
	RoleModerator  Role = "MODERATOR"
	RoleSuperAdmin Role = "SUPER_ADMIN"
	RoleParent     Role = "PARENT"
	RoleSupport    Role = "SUPPORT"
)

type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusInactive  Status = "INACTIVE"
	StatusSuspended Status = "SUSPENDED"
)

type User struct {
	ID            string
	Email         string
	Name          *string
	Username      *string
	Phone         *string
	Avatar        *string
	Role          Role
	Status        Status
	PasswordHash  string
	EmailVerified bool
	Bio           *string
	Balance       float64
	AiCredits     int
	ExamCredits   int
	TotalXP       int
	Level         int
	CurrentStreak int
	LongestStreak int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastLogin     *time.Time
}

type CreateUserInput struct {
	Email    string
	Name     *string
	Username *string
	Phone    *string
	Role     Role
	Password string
}

type UpdateUserInput struct {
	ID       string
	Name     *string
	Username *string
	Phone    *string
	Avatar   *string
	Bio      *string
}

type ListUsersFilter struct {
	Role   *Role
	Status *Status
	Search *string
	Page   int
	Limit  int
}
