package certificate

import (
	"time"
)

// ============================================================================
// Core Entity
// ============================================================================

type Certificate struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	SubjectID   string    `json:"subjectId"`
	SubjectName string    `json:"subjectName"`
	SubjectNameAr *string `json:"subjectNameAr"`
	InstructorName *string `json:"instructorName"`
	UserName    string    `json:"userName"`
	IssuedAt    time.Time `json:"issuedAt"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CertificateWithDetails struct {
	Certificate Certificate `json:"certificate"`
	User        UserInfo    `json:"user"`
	Subject     SubjectInfo `json:"subject"`
}

type UserInfo struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Avatar *string `json:"avatar"`
}

type SubjectInfo struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	NameAr       *string `json:"nameAr"`
	InstructorName *string `json:"instructorName"`
	InstructorId   *string `json:"instructorId"`
	ThumbnailUrl   *string `json:"thumbnailUrl"`
}

// ============================================================================
// Input Types
// ============================================================================

type IssueCertificateInput struct {
	UserID    string
	SubjectID string
}

type ListCertificatesFilter struct {
	UserID    *string
	SubjectID *string
	Page      int
	Limit     int
}

type ListCertificatesResult struct {
	Certificates []Certificate
	Total        int64
	Page         int
	Limit        int
	TotalPages   int64
}