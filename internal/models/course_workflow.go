package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Note: CourseStatus, ValidTransitions, CanTransitionTo, EnrollmentType
// are defined in course_lifecycle.go to avoid duplication.
// This file contains the extended workflow and advanced lesson models.

// SubTopicAudio, SubTopicLink, SubTopicLive, SubTopicDocument
// are defined in subject.go (as extensions to SubTopicType).

// CourseAssistantRole defines the role of a course assistant
type CourseAssistantRole string

const (
	RoleAssistant         CourseAssistantRole = "ASSISTANT"
	RoleCoInstructor      CourseAssistantRole = "CO_INSTRUCTOR"
	RoleTeachingAssistant CourseAssistantRole = "TEACHING_ASSISTANT"
)

// CourseAssistantPermissions defines granular permissions
type CourseAssistantPermissions struct {
	EditContent    bool `json:"editContent"`
	ManageStudents bool `json:"manageStudents"`
	ViewAnalytics  bool `json:"viewAnalytics"`
	ManageQuizzes  bool `json:"manageQuizzes"`
}

// CourseAssistant represents co-instructors and teaching assistants
type CourseAssistant struct {
	ID          string                     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID   string                     `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	UserID      string                     `gorm:"index;type:uuid;column:user_id" json:"userId"`
	Role        CourseAssistantRole        `gorm:"default:'ASSISTANT';column:role" json:"role"`
	Permissions CourseAssistantPermissions `gorm:"type:jsonb;default='{\"edit_content\":false,\"manage_students\":false,\"view_analytics\":true,\"manage_quizzes\":false}';column:permissions" json:"permissions"`
	InvitedBy   *string                    `gorm:"type:uuid;column:invited_by" json:"invitedBy,omitempty"`
	Status      string                     `gorm:"default:'PENDING';column:status" json:"status"` // PENDING, ACTIVE, REVOKED
	InvitedAt   time.Time                  `gorm:"column:invited_at" json:"invitedAt"`
	AcceptedAt  *time.Time                 `gorm:"column:accepted_at" json:"acceptedAt,omitempty"`
	RevokedAt   *time.Time                 `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt   time.Time                  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time                  `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	User    *User    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

func (CourseAssistant) TableName() string {
	return "course_assistants"
}

func (ca *CourseAssistant) BeforeCreate(tx *gorm.DB) error {
	if ca.ID == "" {
		ca.ID = uuid.New().String()
	}
	return nil
}

// CourseReviewSubmission tracks review/approval submissions
type CourseReviewSubmission struct {
	ID               string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID        string     `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	SubmittedBy      string     `gorm:"index;type:uuid;column:submitted_by" json:"submittedBy"`
	Status           string     `gorm:"default:'PENDING';column:status" json:"status"` // PENDING, APPROVED, REJECTED, RESUBMITTED
	ReviewerID       *string    `gorm:"type:uuid;column:reviewer_id" json:"reviewerId,omitempty"`
	ReviewerNotes    *string    `gorm:"column:reviewer_notes" json:"reviewerNotes,omitempty"`
	RejectionReasons []byte     `gorm:"type:jsonb;column:rejection_reasons" json:"-"`
	ReviewedAt       *time.Time `gorm:"column:reviewed_at" json:"reviewedAt,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	// Non-DB fields
	RejectionReasonsParsed []RejectionReason `gorm:"-" json:"rejectionReasons,omitempty"`

	// Relations
	Subject   *Subject `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Submitter *User    `gorm:"foreignKey:SubmittedBy;constraint:OnDelete:CASCADE" json:"submitter,omitempty"`
	Reviewer  *User    `gorm:"foreignKey:ReviewerID;constraint:OnDelete:SET NULL" json:"reviewer,omitempty"`
}

func (CourseReviewSubmission) TableName() string {
	return "course_review_submissions"
}

func (crs *CourseReviewSubmission) BeforeCreate(tx *gorm.DB) error {
	if crs.ID == "" {
		crs.ID = uuid.New().String()
	}
	return nil
}

// RejectionReason represents a specific rejection reason
type RejectionReason struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// SubjectChangelog stores immutable history of course changes.
// Note: This is a separate model from course_lifecycle.go's CourseChangelog
// which uses the "CourseChangelog" table. This one uses "course_changelogs".
type SubjectChangelog struct {
	ID         string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID  string    `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	Version    string    `gorm:"column:version" json:"version"`
	ChangeType string    `gorm:"default:'UPDATE';column:change_type" json:"changeType"` // CREATE, UPDATE, PUBLISH, ARCHIVE, RESTORE, DELETE
	Changes    []byte    `gorm:"type:jsonb;column:changes" json:"-"`
	ChangedBy  *string   `gorm:"type:uuid;column:changed_by" json:"changedBy,omitempty"`
	IPAddress  *string   `gorm:"column:ip_address" json:"ipAddress,omitempty"`
	UserAgent  *string   `gorm:"column:user_agent" json:"userAgent,omitempty"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`

	// Relations
	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
}

func (SubjectChangelog) TableName() string {
	return "course_changelogs"
}

func (cc *SubjectChangelog) BeforeCreate(tx *gorm.DB) error {
	if cc.ID == "" {
		cc.ID = uuid.New().String()
	}
	return nil
}

// DripScheduleType defines how lessons are released
type DripScheduleType string

const (
	DripAbsolute DripScheduleType = "ABSOLUTE" // Fixed date/time
	DripRelative DripScheduleType = "RELATIVE" // Days after enrollment
)

// LessonDripSchedule controls scheduled lesson release
type LessonDripSchedule struct {
	ID                  string           `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubTopicID          string           `gorm:"uniqueIndex;type:uuid;column:sub_topic_id" json:"subTopicId"`
	DripType            DripScheduleType `gorm:"default:'ABSOLUTE';column:drip_type" json:"dripType"`
	ReleaseDate         *time.Time       `gorm:"column:release_date" json:"releaseDate,omitempty"`
	DaysAfterEnrollment *int             `gorm:"column:days_after_enrollment" json:"daysAfterEnrollment,omitempty"`
	IsActive            bool             `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedBy           *string          `gorm:"type:uuid;column:created_by" json:"createdBy,omitempty"`
	CreatedAt           time.Time        `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           time.Time        `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	SubTopic *SubTopic `gorm:"foreignKey:SubTopicID;constraint:OnDelete:CASCADE" json:"subTopic,omitempty"`
}

func (LessonDripSchedule) TableName() string {
	return "lesson_drip_schedules"
}

func (lds *LessonDripSchedule) BeforeCreate(tx *gorm.DB) error {
	if lds.ID == "" {
		lds.ID = uuid.New().String()
	}
	return nil
}

// IsReleased checks if the drip content should be released
func (lds *LessonDripSchedule) IsReleased(enrolledAt time.Time) bool {
	if !lds.IsActive {
		return true
	}
	switch lds.DripType {
	case DripAbsolute:
		if lds.ReleaseDate == nil {
			return true
		}
		return time.Now().After(*lds.ReleaseDate)
	case DripRelative:
		if lds.DaysAfterEnrollment == nil {
			return true
		}
		releaseTime := enrolledAt.AddDate(0, 0, *lds.DaysAfterEnrollment)
		return time.Now().After(releaseTime)
	default:
		return true
	}
}

// VideoChapter represents a timestamped chapter marker
type VideoChapter struct {
	ID          string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubTopicID  string    `gorm:"index;type:uuid;column:sub_topic_id" json:"subTopicId"`
	Title       string    `gorm:"not null;column:title" json:"title"`
	TitleAr     *string   `gorm:"column:title_ar" json:"titleAr,omitempty"`
	TimeSeconds int       `gorm:"default:0;column:time_seconds" json:"timeSeconds"`
	SortOrder   int       `gorm:"default:0;column:sort_order" json:"sortOrder"`
	IsActive    bool      `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	SubTopic *SubTopic `gorm:"foreignKey:SubTopicID;constraint:OnDelete:CASCADE" json:"subTopic,omitempty"`
}

func (VideoChapter) TableName() string {
	return "video_chapters"
}

func (vc *VideoChapter) BeforeCreate(tx *gorm.DB) error {
	if vc.ID == "" {
		vc.ID = uuid.New().String()
	}
	return nil
}

// SubtitleFormat defines supported subtitle formats
type SubtitleFormat string

const (
	SubtitleVTT  SubtitleFormat = "vtt"
	SubtitleSRT  SubtitleFormat = "srt"
	SubtitleJSON SubtitleFormat = "json"
)

// LessonSubtitle represents a subtitle track for a lesson
type LessonSubtitle struct {
	ID                   string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubTopicID           string         `gorm:"uniqueIndex:idx_lesson_subtitle_lang;type:uuid;column:sub_topic_id" json:"subTopicId"`
	Language             string         `gorm:"uniqueIndex:idx_lesson_subtitle_lang;column:language" json:"language"`
	LanguageName         *string        `gorm:"column:language_name" json:"languageName,omitempty"`
	SubtitleUrl          string         `gorm:"not null;column:subtitle_url" json:"subtitleUrl"`
	SubtitleFormat       SubtitleFormat `gorm:"default:'vtt';column:subtitle_format" json:"subtitleFormat"`
	IsDefault            bool           `gorm:"default:false;column:is_default" json:"isDefault"`
	IsForHearingImpaired bool           `gorm:"default:false;column:is_for_hearing_impaired" json:"isForHearingImpaired"`
	CreatedAt            time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            time.Time      `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	SubTopic *SubTopic `gorm:"foreignKey:SubTopicID;constraint:OnDelete:CASCADE" json:"subTopic,omitempty"`
}

func (LessonSubtitle) TableName() string {
	return "lesson_subtitles"
}

func (ls *LessonSubtitle) BeforeCreate(tx *gorm.DB) error {
	if ls.ID == "" {
		ls.ID = uuid.New().String()
	}
	return nil
}

// LessonViewStat stores per-user granular view stats
type LessonViewStat struct {
	ID                  string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubTopicID          string     `gorm:"uniqueIndex:idx_lesson_view_user;type:uuid;column:sub_topic_id" json:"subTopicId"`
	UserID              string     `gorm:"uniqueIndex:idx_lesson_view_user;type:uuid;column:user_id" json:"userId"`
	WatchTimeSeconds    int        `gorm:"default:0;column:watch_time_seconds" json:"watchTimeSeconds"`
	LastPositionSeconds int        `gorm:"default:0;column:last_position_seconds" json:"lastPositionSeconds"`
	Completed           bool       `gorm:"default:false;column:completed" json:"completed"`
	CompletedAt         *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
	Attempts            int        `gorm:"default:0;column:attempts" json:"attempts"`
	MaxPositionSeconds  int        `gorm:"default:0;column:max_position_seconds" json:"maxPositionSeconds"`
	DeviceType          *string    `gorm:"column:device_type" json:"deviceType,omitempty"`
	CreatedAt           time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	SubTopic *SubTopic `gorm:"foreignKey:SubTopicID;constraint:OnDelete:CASCADE" json:"subTopic,omitempty"`
	User     *User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

func (LessonViewStat) TableName() string {
	return "lesson_view_stats"
}

func (lvs *LessonViewStat) BeforeCreate(tx *gorm.DB) error {
	if lvs.ID == "" {
		lvs.ID = uuid.New().String()
	}
	return nil
}

// AvailabilityWindowType defines types of scheduled windows
type AvailabilityWindowType string

const (
	WindowEnrollment AvailabilityWindowType = "ENROLLMENT"
	WindowAccess     AvailabilityWindowType = "ACCESS"
	WindowPublish    AvailabilityWindowType = "PUBLISH"
	WindowUnpublish  AvailabilityWindowType = "UNPUBLISH"
)

// CourseAvailabilityWindow controls scheduled access windows
type CourseAvailabilityWindow struct {
	ID            string                 `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID     string                 `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	WindowType    AvailabilityWindowType `gorm:"default:'PUBLISH';column:window_type" json:"windowType"`
	StartsAt      time.Time              `gorm:"column:starts_at" json:"startsAt"`
	EndsAt        *time.Time             `gorm:"column:ends_at" json:"endsAt,omitempty"`
	IsRepeating   bool                   `gorm:"default:false;column:is_repeating" json:"isRepeating"`
	RepeatPattern *string                `gorm:"column:repeat_pattern" json:"repeatPattern,omitempty"`
	IsActive      bool                   `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedBy     *string                `gorm:"type:uuid;column:created_by" json:"createdBy,omitempty"`
	CreatedAt     time.Time              `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time              `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
}

func (CourseAvailabilityWindow) TableName() string {
	return "course_availability_windows"
}

func (caw *CourseAvailabilityWindow) BeforeCreate(tx *gorm.DB) error {
	if caw.ID == "" {
		caw.ID = uuid.New().String()
	}
	return nil
}
