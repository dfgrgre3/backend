package authdto

// These DTOs describe the public teaching-course JSON contract. They are
// intentionally independent from the GORM Subject model because teaching
// endpoints return a presentation projection, not a database entity.
type TeachingLessonContract struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Duration        string `json:"duration"`
	DurationMinutes int    `json:"durationMinutes"`
	Type            string `json:"type"`
	IsPreview       bool   `json:"isPreview"`
}

type TeachingChapterContract struct {
	ID      string                   `json:"id"`
	Title   string                   `json:"title"`
	Lessons []TeachingLessonContract `json:"lessons"`
}

type TeachingCourseContract struct {
	ID               string                    `json:"id"`
	Title            string                    `json:"title"`
	Description      string                    `json:"description"`
	ShortDescription string                    `json:"shortDescription,omitempty"`
	Thumbnail        string                    `json:"thumbnail"`
	Status           string                    `json:"status"`
	StudentsCount    int                       `json:"studentsCount"`
	LessonsCount     int                       `json:"lessonsCount"`
	Rating           float64                   `json:"rating"`
	Price            float64                   `json:"price"`
	Duration         string                    `json:"duration"`
	Category         string                    `json:"category"`
	CategoryID       *string                   `json:"categoryId,omitempty"`
	Level            string                    `json:"level,omitempty"`
	Language         string                    `json:"language,omitempty"`
	CreatedDate      string                    `json:"createdDate"`
	Chapters         []TeachingChapterContract `json:"chapters"`
}

type TeachingPaginationContract struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type TeachingCoursesListData struct {
	Courses    []TeachingCourseContract   `json:"courses"`
	Pagination TeachingPaginationContract `json:"pagination"`
}

type TeachingCoursesListResponse struct {
	Success bool                    `json:"success"`
	Data    TeachingCoursesListData `json:"data"`
}

type TeachingCourseData struct {
	Course TeachingCourseContract `json:"course"`
}

type TeachingCourseResponse struct {
	Success bool               `json:"success"`
	Data    TeachingCourseData `json:"data"`
}

type TeachingCourseMutationData struct {
	Message  string                  `json:"message,omitempty"`
	Course   *TeachingCourseContract `json:"course,omitempty"`
	Warnings []string                `json:"warnings,omitempty"`
}

type TeachingCourseMutationResponse struct {
	Success bool                       `json:"success"`
	Data    TeachingCourseMutationData `json:"data"`
}

type TeachingCourseDeleteData struct {
	Deleted bool `json:"deleted"`
}

type TeachingCourseDeleteResponse struct {
	Success bool                     `json:"success"`
	Data    TeachingCourseDeleteData `json:"data"`
}
