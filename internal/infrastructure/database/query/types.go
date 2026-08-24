package query

import "time"

// SubjectFilters holds filter parameters for subject queries
type SubjectFilters struct {
	CategoryID  string
	Search      string
	Level       string
	IsPublished *bool
	IsActive    *bool
	Status      string
	IsFeatured  bool
	IsTrending  bool
	IsNew       bool
}

// SubjectListItem represents a lightweight subject for list views
type SubjectListItem struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	NameAr                 string    `json:"nameAr"`
	Code                   *string   `json:"code"`
	Description            *string   `json:"description"`
	Icon                   *string   `json:"icon"`
	Color                  *string   `json:"color"`
	IsActive               bool      `json:"isActive"`
	IsPublished            bool      `json:"isPublished"`
	Price                  float64   `json:"price"`
	Level                  *string   `json:"level"`
	InstructorName         *string   `json:"instructorName"`
	InstructorId           *string   `json:"instructorId"`
	CategoryId             *string   `json:"categoryId"`
	ThumbnailUrl           *string   `json:"thumbnailUrl"`
	TrailerUrl             *string   `json:"trailerUrl"`
	TrailerDurationMinutes *int      `json:"trailerDurationMinutes"`
	DurationHours          *int      `json:"durationHours"`
	Requirements           *string   `json:"requirements"`
	LearningObjectives     *string   `json:"learningObjectives"`
	SeoTitle               *string   `json:"seoTitle"`
	SeoDescription         *string   `json:"seoDescription"`
	Slug                   *string   `json:"slug"`
	Rating                 *float64  `json:"rating"`
	EnrolledCount          int       `json:"enrolledCount"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
	TopicCount             int64     `json:"topicCount"`
}

// SubjectDetail represents the full details of a subject including curriculum
type SubjectDetail struct {
	ID                     string               `json:"id"`
	Name                   string               `json:"name"`
	NameAr                 string               `json:"nameAr"`
	Code                   *string              `json:"code"`
	Description            *string              `json:"description"`
	Icon                   *string              `json:"icon"`
	Color                  *string              `json:"color"`
	IsActive               bool                 `json:"isActive"`
	IsPublished            bool                 `json:"isPublished"`
	Price                  float64              `json:"price"`
	Level                  *string              `json:"level"`
	InstructorName         *string              `json:"instructorName"`
	InstructorId           *string              `json:"instructorId"`
	CategoryId             *string              `json:"categoryId"`
	ThumbnailUrl           *string              `json:"thumbnailUrl"`
	TrailerUrl             *string              `json:"trailerUrl"`
	TrailerDurationMinutes *int                 `json:"trailerDurationMinutes"`
	DurationHours          *int                 `json:"durationHours"`
	Requirements           *string              `json:"requirements"`
	LearningObjectives     *string              `json:"learningObjectives"`
	SeoTitle               *string              `json:"seoTitle"`
	SeoDescription         *string              `json:"seoDescription"`
	Slug                   *string              `json:"slug"`
	Rating                 *float64             `json:"rating"`
	EnrolledCount          int                  `json:"enrolledCount"`
	CreatedAt              time.Time            `json:"createdAt"`
	UpdatedAt              time.Time            `json:"updatedAt"`
	Status                 *string              `json:"status"`
	Version                *int                 `json:"version"`
	ShortDescription       *string              `json:"shortDescription"`
	LongDescription        *string              `json:"longDescription"`
	IsFeatured             bool                 `json:"isFeatured"`
	IsTrending             bool                 `json:"isTrending"`
	IsNew                  bool                 `json:"isNew"`
	IsFree                 bool                 `json:"isFree"`
	HasCertificate         bool                 `json:"hasCertificate"`
	AvailableFrom          *time.Time           `json:"availableFrom"`
	AvailableUntil         *time.Time           `json:"availableUntil"`
	NewUntil               *time.Time           `json:"newUntil"`
	CoursePrerequisites    []string             `json:"coursePrerequisites"`
	TargetAudience         []string             `json:"targetAudience"`
	WhatYouLearn           []string             `json:"whatYouLearn"`
	Type                   string               `json:"type"`
	Curriculum             []TopicWithSubTopics `json:"curriculum"`
}

// TopicWithSubTopics represents a topic with its nested subtopics
type TopicWithSubTopics struct {
	ID          string           `json:"id"`
	SubjectID   string           `json:"subjectId"`
	Title       string           `json:"title"`
	Description *string          `json:"description"`
	Order       int              `json:"order"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	SubTopics   []SubTopicDetail `json:"subTopics"`
}

// SubTopicDetail represents the detail of a single subtopic
type SubTopicDetail struct {
	ID                   string     `json:"id"`
	TopicID              string     `json:"topicId"`
	Title                string     `json:"title"`
	Description          *string    `json:"description"`
	VideoUrl             *string    `json:"videoUrl"`
	AudioUrl             *string    `json:"audioUrl"`
	AudioDurationSeconds *int       `json:"audioDurationSeconds"`
	ExternalLinkUrl      *string    `json:"externalLinkUrl"`
	ExternalLinkTitle    *string    `json:"externalLinkTitle"`
	Type                 string     `json:"type"`
	IsFree               bool       `json:"isFree"`
	Order                int        `json:"order"`
	DurationMinutes      int        `json:"durationMinutes"`
	ExamID               *string    `json:"examId"`
	IsDripEnabled        bool       `json:"isDripEnabled"`
	DripReleaseDate      *time.Time `json:"dripReleaseDate"`
	IsContentProtected   bool       `json:"isContentProtected"`
	ViewCount            int        `json:"viewCount"`
	CompletionCount      int        `json:"completionCount"`
	SubtitleUrls         []string   `json:"subtitleUrls"`
	VideoChaptersData    []byte     `json:"videoChaptersData"`
}
