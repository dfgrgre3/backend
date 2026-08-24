package protected

import (
	"sort"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"time"

	"github.com/gin-gonic/gin"
)

// Lesson represents the lesson structure returned to the frontend
type Lesson struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	VideoUrl          string `json:"videoUrl"`
	AudioUrl          string `json:"audioUrl,omitempty"`
	AudioDuration     int    `json:"audioDuration,omitempty"`
	ExternalLinkUrl   string `json:"externalLinkUrl,omitempty"`
	ExternalLinkTitle string `json:"externalLinkTitle,omitempty"`
	Type              string `json:"type"`
	IsFree            bool   `json:"isFree"`
	Order             int    `json:"order"`
	DurationMinutes   int    `json:"durationMinutes"`
	ExamID            string `json:"examId,omitempty"`
	// Advanced fields
	IsDripEnabled      bool                       `json:"isDripEnabled,omitempty"`
	DripReleaseDate    string                     `json:"dripReleaseDate,omitempty"`
	IsContentProtected bool                       `json:"isContentProtected,omitempty"`
	HasSubtitles       bool                       `json:"hasSubtitles,omitempty"`
	HasChapters        bool                       `json:"hasChapters,omitempty"`
	ViewCount          int                        `json:"viewCount,omitempty"`
	CompletionCount    int                        `json:"completionCount,omitempty"`
	Attachments        []models.LessonAttachment  `json:"attachments,omitempty"`
}

func GetCourseLessons(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	id := c.Param("id")
	var subject models.Subject

	query := database.Preload(preloadAdvanced)
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "fetching course lessons")
		return
	}

	sortAttachments := func(atts []models.LessonAttachment) []models.LessonAttachment {
		sort.Slice(atts, func(i, j int) bool { return atts[i].CreatedAt.Before(atts[j].CreatedAt) })
		return atts
	}

	var lessons []Lesson
	for _, topic := range subject.Topics {
		for _, st := range topic.SubTopics {
			l := Lesson{
				ID:                 st.ID,
				Title:              st.Title,
				Description:        stringOrEmpty(st.Description),
				Type:               string(st.Type),
				IsFree:             st.IsFree,
				Order:              st.Order,
				DurationMinutes:    st.DurationMinutes,
				ExamID:             stringOrEmpty(st.ExamID),
				IsDripEnabled:      st.IsDripEnabled,
				IsContentProtected: st.IsContentProtected,
				ViewCount:          st.ViewCount,
				CompletionCount:    st.CompletionCount,
			}
			// SECURITY: this endpoint is public and unauthenticated (used for
			// course preview pages), so the actual playable content — video/
			// audio URLs, external links, and attachments — must only be
			// exposed for free-preview lessons. Otherwise anyone could list a
			// course's lessons and extract direct links to paid content
			// without enrolling. Enrolled users get real access via the
			// separate GetCourseLessonsWithAccess endpoint, which checks
			// enrollment/drip eligibility per lesson.
			if st.IsFree {
				l.VideoUrl = stringOrEmpty(st.VideoUrl)
				l.AudioUrl = stringOrEmpty(st.AudioUrl)
				l.AudioDuration = st.AudioDurationSeconds
				l.ExternalLinkUrl = stringOrEmpty(st.ExternalLinkUrl)
				l.ExternalLinkTitle = stringOrEmpty(st.ExternalLinkTitle)
				l.Attachments = sortAttachments(st.Attachments)
				if st.DripReleaseDate != nil {
					l.DripReleaseDate = st.DripReleaseDate.Format(time.RFC3339)
				}
			}
			// Check for subtitles and chapters
			if len(st.SubtitleUrls) > 0 {
				l.HasSubtitles = true
			}
			if len(st.VideoChaptersData) > 0 {
				l.HasChapters = true
			}
			lessons = append(lessons, l)
		}
	}

	api_response.Success(c, gin.H{
		"lessons": lessons,
		"course": gin.H{
			"id":        subject.ID,
			"title":     subject.Name,
			"titleAr":   subject.NameAr,
			"status":    subject.Status,
			"type":      string(subject.Type),
			"thumbnail": subject.ThumbnailUrl,
		},
	})
}
