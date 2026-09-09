package authdto

import models "thanawy-backend/internal/domain/common"

// Pagination describes the stable pagination metadata returned by catalog
// endpoints. It intentionally mirrors the public JSON contract rather than
// exposing the infrastructure response package to application DTOs.
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

type CourseListData struct {
	Items      []models.Subject `json:"items"`
	Pagination Pagination       `json:"pagination"`
	Offset     int              `json:"offset"`
}

type CourseListResponse struct {
	Success bool           `json:"success"`
	Data    CourseListData `json:"data"`
}

type CourseDetailData struct {
	Subject models.Subject `json:"subject"`
	Course  models.Subject `json:"course"`
}

type CourseDetailResponse struct {
	Success    bool               `json:"success"`
	Data       CourseDetailData   `json:"data"`
	Subject    models.Subject     `json:"subject"`
	Enrollment *models.Enrollment `json:"enrollment,omitempty"`
}
